#!/usr/bin/env python3
"""Jowitt-style LP pump scheduler, version 3.

PURPOSE
-------
This program schedules four pumps in a small water-supply system while
respecting tank-storage and hydraulic constraints.  It combines two tools:

1. a *linear program (LP)*, which chooses how much of each hour should be
   allocated to each possible pump combination; and
2. EPANET, which represents the nonlinear hydraulic behaviour of the real
   water network and checks that the LP schedule is physically feasible.

The code is written as a rolling-horizon controller: at each real hour it
optimizes several hours ahead, executes only the first-hour decision, observes
the resulting hydraulic state, and optimizes again.  Readers do not need a
background in control or optimization; the main equations are summarized
below and referenced throughout the code as Eq. (1), Eq. (2), etc.

MATHEMATICAL MODEL USED BY THE CODE
-----------------------------------
Notation used below:

    h       forecast-hour index, h = 0, ..., H-1
    m       pump-mode index, m = 0, ..., 15
    i       tank index, i = 1, 2
    f[h,m]  fraction of forecast hour h spent in pump mode m
    V[h,i]  tank volume at the end of forecast hour h [m^3]
    q[h,m,i] net inflow to tank i when mode m is used [m^3/h]
    D[h,i]  forecast demand assigned to tank/zone i [m^3/h]
    P[h,m]  electrical power of mode m [kW]
    c[h]    electricity tariff in hour h [currency/kWh]

There are four binary pumps, hence 2^4 = 16 possible pump modes.  Rather than
choosing exactly one mode for an entire hour, the LP may divide the hour among
several modes.  For example, f=0.25 means 15 minutes of operation.

Eq. (1) -- complete allocation of every hour

    sum_m f[h,m] = 1,       0 <= f[h,m] <= 1

Eq. (2) -- tank water balance / conservation of mass

    V[h,i] = V[h-1,i] + sum_m q[h,m,i] f[h,m] - D[h,i]

For h=0, V[h-1,i] is replaced by the measured current tank volume.  Because
the scheduling interval is one hour, a flow in m^3/h multiplied by an hourly
fraction directly gives a volume in m^3.

Eq. (3) -- electricity cost represented in the LP

    J_energy = sum_h sum_m c[h] P[h,m] f[h,m]

Eq. (4) -- physical/operational tank-volume bounds

    V_min[i] <= V[h,i] <= V_max[i]

The lower bound includes a small physical safety margin; the upper bound is
kept below the physical maximum by the reserve margin used in this study.

Eq. (5) -- preferred reserve, implemented as a *soft* constraint

    V[h,i] + r[h,i] >= V_reserve[i],     r[h,i] >= 0

The nonnegative slack r allows a preferred reserve to be violated when
necessary, but Eq. (7) penalizes such violations very strongly.

Eq. (6) -- terminal-storage target, also soft

    V[H-1,i] + z[i] >= V_target[i],      z[i] >= 0

Eq. (7) -- complete LP objective

    minimize  J_energy
            + lambda_terminal * sum_i z[i]
            + lambda_reserve  * sum_h sum_i r[h,i]

The LP is a simplified model because pump flow and power vary with hydraulic
head.  EPANET therefore supplies q and P at representative future tank levels.
The LP solution is then replayed through EPANET and, if necessary, recalibrated.

Eq. (8) -- successive hydraulic refinement

    trial_next = (1-alpha) * trial_current + alpha * replay_midpoints

where ``trial`` is the trajectory at which pump-mode flow/power coefficients
are evaluated and alpha is a relaxation factor.  Version 2 adapts alpha when
requested.

Eq. (9) -- LP/EPANET trajectory mismatch

    e = max_h,i | h_EPANET[h,i] - h_LP[h,i] |

Refinement stops when e is below the requested tolerance or when the maximum
number of iterations has been reached.

Eq. (10) -- rolling-horizon / receding-horizon principle

    optimize H hours -> execute first hour -> observe new state -> optimize again

Only the first-hour schedule is committed.  This feedback structure allows the
controller to respond to realized demand that differs from the forecast.

VERSION-2 ENHANCEMENTS
----------------------
Compared with the minimal v1 implementation, v2 adds:

  1. warm-starting of the hydraulic trial trajectory between MPC steps;
  2. adaptive relaxation in Eq. (8);
  3. within-hour sequence ranking using actual EPANET energy, then pump starts,
     then tank-boundary clearance;
  4. a configurable minimum nonzero mode duration;
  5. a configurable service-pressure threshold; and
  6. a stronger, configurable terminal-storage penalty.

The underlying optimization remains a continuous run-time LP over the 16 pump
modes.  EPANET supplies local mode coefficients and validates execution.
"""

from __future__ import annotations

# Standard-library imports.
# argparse: command-line options; itertools: pump-state combinations/permutations;
# json/pandas: result output; tempfile/shutil: isolated EPANET working copy.
import argparse
import itertools
import json
import shutil
import tempfile
from pathlib import Path

# Numerical/data libraries.
# NumPy stores trajectories and matrices, pandas reads/writes tabular inputs, and
# SciPy's linprog solves the linear optimization problem defined by Eqs. (1)-(7).
import numpy as np
import pandas as pd
from scipy.optimize import linprog

# EPyT is the Python interface to EPANET.  Keeping this import optional lets the
# algebraic/unit tests run even on machines where EPANET is not installed.
try:
    from epyt import epanet
except ImportError:
    epanet = None


# -----------------------------------------------------------------------------
# Network structure and controller constants
# -----------------------------------------------------------------------------
PUMPS = ["pump1", "pump2", "pump3", "booster"]
TANKS = ["t1", "t2"]
DEMAND_NODES = ["12", "11"]
# All binary ON/OFF combinations are enumerated once.
# WELL_MODES contains 2^3 = 8 combinations of the three well pumps.
# MODES contains 2^4 = 16 combinations of all pumps, including the booster.
# A row such as [1, 0, 1, 1] means pumps 1, 3 and the booster are ON.
WELL_MODES = np.array(list(itertools.product([0, 1], repeat=3)), dtype=int)
MODES = np.array(list(itertools.product([0, 1], repeat=4)), dtype=int)

# Preferred storage reserve above the tank minimum [m].  This is soft in the LP
# through Eq. (5), while PHYSICAL_MARGIN_M contributes to the hard lower bound.
RESERVE_M = 0.30
PHYSICAL_MARGIN_M = 0.05
RESERVE_PENALTY = 1000.0
DEFAULT_TERMINAL_PENALTY = 10.0
DEFAULT_MIN_PRESSURE_M = 20.0
DEFAULT_MIN_SEGMENT_S = 300

def load_data(data_dir: Path):
    """Read the small set of external data required by the controller.

    Parameters
    ----------
    data_dir : Path
        Folder containing ``demands.csv``, ``tariffs.csv`` and ``tanks.csv``.

    Returns
    -------
    nominal_demands : ndarray, shape (24, 2)
        Hourly demand pattern for the two demand zones [m^3/h].
    tariffs : ndarray, shape (24,)
        Electricity price for each hour of the daily tariff cycle.
    tanks : pandas.DataFrame
        Tank geometry and initial/minimum/maximum levels.

    Notes
    -----
    The demand and tariff files describe one 24-h pattern.  ``run()`` repeats
    these patterns when simulations extend beyond one day.
    """
    demand = pd.read_csv(data_dir / "demands.csv").sort_values("time")
    tariff = pd.read_csv(data_dir / "tariffs.csv").sort_values("time")
    tanks = pd.read_csv(data_dir / "tanks.csv", index_col=0).loc[TANKS]
    return demand[TANKS].to_numpy(float), tariff["tariff"].to_numpy(float), tanks


class WaterNetwork:
    """Hydraulic-model interface between the optimizer and EPANET.

    The LP itself does not solve the nonlinear hydraulic equations.  This class
    asks EPANET two different questions:

    1. *Operating-point question*: for given tank levels, demands and pump
       states, what are the pump flows and powers?  Those values become q and P
       in Eqs. (2) and (3).
    2. *Dynamic validation question*: if a selected mode is actually operated
       for a specified number of seconds, how do tank levels, pressures and
       energy use evolve?

    Keeping these responsibilities here makes the optimization code easier to
    read for users who are unfamiliar with EPANET.
    """

    def __init__(self, inp_file: Path, tanks: pd.DataFrame, work_dir: str, min_pressure_m: float):
        """Create an isolated EPANET model and cache useful node/link indices."""
        if epanet is None:
            raise RuntimeError("Install epyt==2.3.5.2 to run the hydraulic model")

        # Work on a temporary copy of the INP file.  EPANET/EPyT may create or
        # modify companion files, so the user's source model is left untouched.
        local_inp = Path(work_dir) / "network.inp"
        shutil.copy2(inp_file, local_inp)
        self.d = epanet(str(local_inp))
        self.min_pressure_m = float(min_pressure_m)

        nodes = self.d.getNodeNameID()
        links = self.d.getLinkNameID()
        self.tank_idx = np.array([nodes.index(name) for name in TANKS])
        self.demand_idx = np.array([nodes.index(name) for name in DEMAND_NODES])
        self.pump_idx = np.array([links.index(name) for name in PUMPS])
        self.junction_idx = np.array(
            [i for i, kind in enumerate(self.d.getNodeType()) if kind == "JUNCTION"]
        )

        # Tank cross-sectional area converts between the LP state V [m^3] and
        # the more intuitive/EPANET state h [m]:  V = A h, hence h = V/A.
        self.area = np.pi * tanks["diameter"].to_numpy(float) ** 2 / 4
        self.minimum = tanks["min_level"].to_numpy(float)
        self.maximum = tanks["max_level"].to_numpy(float)
        self.initial = tanks["init_level"].to_numpy(float)

        # Demand is supplied explicitly by this controller, so existing EPANET
        # patterns are neutralized to avoid multiplying our demand twice.
        for pattern in range(1, self.d.getPatternCount() + 1):
            self.d.setPattern(pattern, [1.0])
        self.d.setTimeHydraulicStep(300)
        self.d.setTimeReportingStep(300)
        self.d.setOptionsAccuracyValue(0.0001)

    def close(self):
        """Release the EPANET model from memory."""
        self.d.unload()

    def configure(self, levels, demand, pump_state, duration_s):
        """Write one hydraulic scenario into EPANET.

        ``levels`` [m] are the starting tank depths, ``demand`` [m^3/h] is the
        zone demand, ``pump_state`` is a four-element 0/1 vector, and
        ``duration_s`` specifies how long the constant mode is simulated.
        """
        self.d.setNodeTankInitialLevel((self.tank_idx + 1).tolist(), levels.tolist())

        base_demand = np.zeros(len(self.d.getNodeNameID()))
        base_demand[self.demand_idx] = demand
        self.d.setNodeBaseDemands(base_demand.tolist())

        pump_ids = (self.pump_idx + 1).tolist()
        self.d.setLinkInitialSetting(pump_ids, [1.0] * 4)
        self.d.setLinkInitialStatus(pump_ids, np.asarray(pump_state, int).tolist())
        self.d.setTimeSimulationDuration(int(duration_s))

    def operating_point(self, levels, demand, pump_state):
        """Return pump flow and power at a fixed hydraulic operating point.

        This is a zero-duration hydraulic solve: it does not advance the tank
        state.  Its purpose is to obtain the local coefficients q[h,m,i] and
        P[h,m] used in the LP mass balance (Eq. 2) and cost (Eq. 3).
        """
        self.configure(levels, demand, pump_state, 0)
        self.d.openHydraulicAnalysis()
        try:
            self.d.initializeHydraulicAnalysis(10)
            self.d.runHydraulicAnalysis()
            flow = np.asarray(self.d.getLinkFlows(), float)[self.pump_idx]
            power = np.asarray(self.d.getLinkEnergy(), float)[self.pump_idx]
            return flow, power
        finally:
            self.d.closeHydraulicAnalysis()

    def mode_table(self, trial_levels, forecast):
        """Build the LP hydraulic coefficient tables for the whole horizon.

        Parameters
        ----------
        trial_levels : ndarray, shape (H, 2)
            Representative tank levels at which each future hour is linearized.
        forecast : ndarray, shape (H, 2)
            Forecast demand for each future hour.

        Returns
        -------
        net : ndarray, shape (H, 16, 2)
            ``net[h,m,i]`` is the net pumping inflow to tank i [m^3/h] used as
            q[h,m,i] in Eq. (2).  Consumer demand is deliberately excluded here
            because Eq. (2) subtracts it separately.
        power : ndarray, shape (H, 16)
            ``power[h,m]`` is total electrical power [kW] for mode m and becomes
            P[h,m] in Eq. (3).

        The specific network has three wells feeding Tank 1 and a booster
        transferring water from Tank 1 to Tank 2.  That topology lets the code
        evaluate 8 well combinations plus one booster operating point and then
        compose the 16 full modes efficiently.
        """
        H = len(forecast)
        net = np.zeros((H, 16, 2))
        power = np.zeros((H, 16))

        for h in range(H):
            well_flow = np.zeros(8)
            well_power = np.zeros(8)

            for w, wells in enumerate(WELL_MODES):
                q, p = self.operating_point(trial_levels[h], forecast[h], [*wells, 0])
                well_flow[w] = q[:3].sum()
                well_power[w] = p[:3].sum()

            q_booster, p_booster = self.operating_point(
                trial_levels[h], forecast[h], [0, 0, 0, 1]
            )

            for m, state in enumerate(MODES):
                w = 4 * state[0] + 2 * state[1] + state[2]
                # Booster flow is a transfer: it leaves Tank 1 and enters Tank 2.
                # Therefore q_T1 = well inflow - booster transfer and
                # q_T2 = booster transfer.  These are the q terms in Eq. (2).
                transfer = q_booster[3] * state[3]
                net[h, m] = [well_flow[w] - transfer, transfer]
                power[h, m] = well_power[w] + p_booster[3] * state[3]

        return net, power

    def run_segment(self, levels, demand, pump_state, seconds):
        """Dynamically simulate one constant pump mode using EPANET.

        Unlike ``operating_point()``, this advances the hydraulic state through
        time.  It is used to validate that a proposed schedule respects the true
        nonlinear network rather than only the LP approximation.

        Returns a dictionary containing final/minimum/maximum tank levels,
        minimum junction pressure, energy consumed by each pump, and a Boolean
        hydraulic-feasibility flag.
        """
        self.configure(levels, demand, pump_state, seconds)

        times, depths, powers, statuses, pressures = [], [], [], [], []
        self.d.openHydraulicAnalysis()
        try:
            self.d.initializeHydraulicAnalysis(10)
            while True:
                now = self.d.runHydraulicAnalysis()
                pressure = np.asarray(self.d.getNodePressure(), float)

                times.append(now)
                depths.append(pressure[self.tank_idx])
                pressures.append(pressure[self.junction_idx])
                powers.append(np.asarray(self.d.getLinkEnergy(), float)[self.pump_idx])
                statuses.append(np.asarray(self.d.getLinkStatus())[self.pump_idx])

                if now >= seconds:
                    break

                self.d.setTimeHydraulicStep(min(300, int(seconds - now)))
                if self.d.nextHydraulicAnalysisStep() <= 0:
                    raise RuntimeError("Hydraulic simulation stopped before a switch")
        finally:
            self.d.closeHydraulicAnalysis()
            self.d.setTimeHydraulicStep(300)

        times = np.asarray(times, float)
        depths = np.asarray(depths, float)
        powers = np.asarray(powers, float)
        statuses = np.asarray(statuses)
        pressures = np.asarray(pressures, float)
        dt_h = np.diff(times) / 3600.0

        minimum_pressure = float(pressures.min()) if pressures.size else np.inf
        # A segment is physically acceptable only if tank levels remain inside
        # their true limits, pressure stays above the selected service threshold,
        # and EPANET actually keeps the commanded pump states.
        feasible = (
            np.all(depths >= self.minimum - 1e-4)
            and np.all(depths <= self.maximum + 1e-4)
            and minimum_pressure >= self.min_pressure_m
            and np.all(statuses[:-1] == np.asarray(pump_state))
        )

        return {
            "levels": depths[-1],
            "minimum": depths.min(axis=0),
            "maximum": depths.max(axis=0),
            "minimum_pressure": minimum_pressure,
            "energy": np.sum(powers[:-1] * dt_h[:, None], axis=0),
            "feasible": bool(feasible),
        }


def solve_lp(levels, forecast, prices, net, power, network, terminal_penalty):
    """Solve the Jowitt-style linear program for one forecast horizon.

    The optimization variables are packed into one long vector ``x`` because
    SciPy's ``linprog`` expects that form.  Index arrays ``f``, ``v``, ``z`` and
    ``r`` let the code address meaningful blocks of that vector:

    * f[h,m] : fraction of hour h assigned to pump mode m, Eq. (1)
    * v[h,i] : end-of-hour tank volume [m^3], Eq. (2)
    * z[i]   : terminal-volume shortfall slack, Eq. (6)
    * r[h,i] : preferred-reserve shortfall slack, Eq. (5)

    Returns
    -------
    fractions : ndarray, shape (H, 16)
        Optimal mode fractions f[h,m].
    predicted_levels : ndarray, shape (H, 2)
        LP-predicted end-of-hour tank levels [m].  ``linprog`` solves for
        volumes; division by tank area converts V to h using V = A h.
    """
    H = len(forecast)
    n_f = 16 * H
    n_v = 2 * H
    n = n_f + n_v + 2 + 2 * H

    # Map meaningful variable names onto positions in linprog's flat x vector.
    # For H=12 there are 192 fraction variables and 24 tank-volume variables.
    f = np.arange(n_f).reshape(H, 16)
    v = np.arange(n_f, n_f + n_v).reshape(H, 2)
    z = np.arange(n_f + n_v, n_f + n_v + 2)
    r = np.arange(n_f + n_v + 2, n).reshape(H, 2)

    # Objective vector c for min c^T x.
    # Eq. (3): tariff [currency/kWh] * power [kW] * hourly fraction [h]
    # gives operating cost.  Eq. (7) then adds penalties on z and r.
    c = np.zeros(n)
    c[f] = prices[:, None] * power
    c[z] = terminal_penalty
    c[r] = RESERVE_PENALTY

    # Equality constraints A_eq x = b_eq implement Eqs. (1) and (2).
    A_eq, b_eq = [], []
    for h in range(H):
        # Eq. (1): all fractions within one hour must sum exactly to 1.
        row = np.zeros(n)
        row[f[h]] = 1.0
        A_eq.append(row)
        b_eq.append(1.0)

        # Eq. (2): mass balance for each tank.  Written in linprog form,
        # V[h,i] - sum_m q[h,m,i] f[h,m] - V[h-1,i] = -D[h,i].
        for tank in range(2):
            row = np.zeros(n)
            row[v[h, tank]] = 1.0
            row[f[h]] = -net[h, :, tank]
            rhs = -forecast[h, tank]

            if h == 0:
                # First forecast step starts from the *measured/current* tank
                # level.  Multiplying h [m] by A [m^2] gives V [m^3].
                rhs += levels[tank] * network.area[tank]
            else:
                row[v[h - 1, tank]] = -1.0

            A_eq.append(row)
            b_eq.append(rhs)

    # Inequalities A_ub x <= b_ub implement the two soft storage policies:
    # terminal target (Eq. 6) and preferred reserve (Eq. 5).  Signs look
    # inverted because linprog accepts <=, e.g. V+z >= target is written
    # -V-z <= -target.
    A_ub = np.zeros((2 + 2 * H, n))
    b_ub = np.r_[
        -network.initial * network.area,
        np.tile(-(network.minimum + RESERVE_M) * network.area, H),
    ]

    for tank in range(2):
        A_ub[tank, v[-1, tank]] = -1.0
        A_ub[tank, z[tank]] = -1.0

    for h in range(H):
        for tank in range(2):
            row = 2 + 2 * h + tank
            A_ub[row, v[h, tank]] = -1.0
            A_ub[row, r[h, tank]] = -1.0

    # Eq. (4): hard tank bounds.  These are true variable bounds, so the LP
    # cannot buy its way through them with slack.  The preferred reserve in
    # Eq. (5) remains soft and is handled separately above.
    volume_bounds = [
        (
            (network.minimum[tank] + PHYSICAL_MARGIN_M) * network.area[tank],
            (network.maximum[tank] - RESERVE_M) * network.area[tank],
        )
        for _ in range(H)
        for tank in range(2)
    ]
    bounds = [(0.0, 1.0)] * n_f + volume_bounds + [(0.0, None)] * (2 + 2 * H)

    # HiGHS returns the globally optimal solution of this *linear surrogate*.
    # This is not a proof of global optimality for the full nonlinear hydraulic
    # pump-scheduling problem; EPANET replay/refinement handles that mismatch.
    result = linprog(
        c,
        A_ub=A_ub,
        b_ub=b_ub,
        A_eq=np.asarray(A_eq),
        b_eq=np.asarray(b_eq),
        bounds=bounds,
        method="highs",
    )
    if not result.success:
        raise RuntimeError(result.message)

    # ``result.x`` contains every LP decision variable in one flat vector.
    # Extract f directly.  Extract v [m^3] and convert it to level [m] using
    # h = V/A so the prediction can be compared directly with EPANET (Eq. 9).
    return result.x[f], result.x[v] / network.area


def fractions_to_seconds(fractions):
    """Convert LP mode fractions into executable integer-second durations.

    The LP works in fractions of a one-hour interval.  EPANET execution requires
    actual durations.  The largest-remainder rounding below guarantees that all
    mode durations still sum to exactly 3600 s after integer rounding.
    """
    raw = np.maximum(fractions, 0.0)
    if raw.sum() <= 0:
        raise ValueError("At least one mode fraction must be positive")
    raw = 3600.0 * raw / raw.sum()
    seconds = np.floor(raw).astype(int)
    remaining = 3600 - seconds.sum()
    seconds[np.argsort(-(raw - seconds), kind="stable")[:remaining]] += 1
    return seconds


def enforce_minimum_segment(seconds, min_segment_s, mode_cost=None):
    """Repair impractically short nonzero operating segments.

    This is an engineering post-processing step; it is not part of the original
    LP equations.  A tiny LP fraction can otherwise become, for example, a
    67-second pump mode.  V2 transfers such time to the closest active pump mode
    in Hamming distance (fewest ON/OFF differences), using predicted mode cost
    as a tie breaker.

    Remove nonzero segments shorter than ``min_segment_s``.

    Time from a short mode is transferred to the most similar active mode.
    Hamming distance between pump states is minimized first; mode cost is used
    as the tie breaker. Total duration remains exactly 3600 s.
    """
    seconds = np.asarray(seconds, int).copy()
    min_segment_s = int(min_segment_s)
    if min_segment_s <= 0:
        return seconds

    mode_cost = np.zeros(16) if mode_cost is None else np.asarray(mode_cost, float)

    while True:
        short = np.flatnonzero((seconds > 0) & (seconds < min_segment_s))
        active = np.flatnonzero(seconds > 0)
        if len(short) == 0 or len(active) <= 1:
            break

        # Remove the shortest segment first.
        m = int(short[np.argmin(seconds[short])])
        receivers = active[active != m]
        distances = np.sum(MODES[receivers] != MODES[m], axis=1)
        best_distance = distances.min()
        candidates = receivers[distances == best_distance]
        receiver = int(candidates[np.argmin(mode_cost[candidates])])

        seconds[receiver] += seconds[m]
        seconds[m] = 0

    if seconds.sum() != 3600:
        raise RuntimeError("Minimum-duration repair changed the one-hour duration")
    return seconds


def simulate_sequence(network, levels, demand, seconds, order, previous_state):
    """Execute one complete hour as an ordered sequence of pump modes.

    The LP determines *how long* each mode should operate but does not determine
    chronological ordering within the hour.  Because hydraulics depend on the
    evolving tank levels, two different orderings of the same durations can have
    different energy use and feasibility.  This function performs the nonlinear
    EPANET replay used to resolve that ambiguity.
    """
    state = levels.copy()
    minimum = state.copy()
    maximum = state.copy()
    energy = np.zeros(4)
    starts = np.zeros(4, dtype=int)
    current = previous_state.copy()
    minimum_pressure = np.inf

    for mode in order:
        if seconds[mode] <= 0:
            continue
        pump_state = MODES[mode]
        # Count OFF->ON transitions.  ``previous_state`` makes switching
        # accounting continuous across the boundary between two hours.
        starts += (pump_state == 1) & (current == 0)
        current = pump_state.copy()

        result = network.run_segment(state, demand, pump_state, int(seconds[mode]))
        if not result["feasible"]:
            return None

        state = result["levels"]
        minimum = np.minimum(minimum, result["minimum"])
        maximum = np.maximum(maximum, result["maximum"])
        minimum_pressure = min(minimum_pressure, result["minimum_pressure"])
        energy += result["energy"]

    return state, minimum, maximum, energy, starts, current, minimum_pressure


def choose_sequence(
    network,
    levels,
    demand,
    fractions,
    previous_state,
    min_segment_s=DEFAULT_MIN_SEGMENT_S,
    mode_cost=None,
    ranking="energy",
):
    """Choose a feasible chronological ordering of the LP mode durations.

    The LP supplies fractions only; mathematically, Eq. (2) treats their hourly
    contribution as additive and therefore does not encode whether mode A runs
    before or after mode B.  EPANET does care about this order because the tank
    head changes continuously.

    For up to six active modes every permutation is checked.  Beyond six modes,
    exhaustive factorial search becomes expensive, so forward/reverse cyclic
    orderings are used as a practical heuristic.

    ``ranking="energy"`` (v2 default) ranks feasible candidates by actual
    EPANET energy, then starts, then physical tank-boundary clearance.
    ``ranking="clearance"`` reproduces the v1 ordering rule: maximize
    clearance first, then minimize starts.
    """
    seconds = fractions_to_seconds(fractions)
    seconds = enforce_minimum_segment(seconds, min_segment_s, mode_cost)
    active = tuple(np.flatnonzero(seconds))

    if len(active) <= 6:
        candidates = itertools.permutations(active)
    else:
        candidates = (
            order[k:] + order[:k]
            for order in (active, active[::-1])
            for k in range(len(active))
        )

    best = None
    best_key = None
    for order in candidates:
        result = simulate_sequence(network, levels, demand, seconds, order, previous_state)
        if result is None:
            continue

        _, minimum, maximum, energy, starts, _, _ = result
        clearance = min(
            np.min(minimum - network.minimum),
            np.min(network.maximum - maximum),
        )
        if ranking == "energy":
            # Lexicographic v2 rule: lower true EPANET energy first, then fewer
            # starts, then larger hydraulic clearance from tank limits.
            key = (float(energy.sum()), int(starts.sum()), -float(clearance))
        elif ranking == "clearance":
            key = (-float(clearance), int(starts.sum()))
        else:
            raise ValueError("ranking must be 'energy' or 'clearance'")

        if best is None or key < best_key:
            best = (seconds, order, result)
            best_key = key

    if best is None:
        raise RuntimeError("No hydraulically feasible ordering of the LP modes")
    return best


def warm_start_trial(current_levels, previous_replay, horizon):
    """Warm-start the next hydraulic linearization from the previous replay.

    In a rolling-horizon controller, consecutive optimization problems overlap
    heavily.  At hour k we already predicted most of the states needed at hour
    k+1.  Reusing that trajectory is usually a better starting point than
    assuming every future tank level equals the current level.  This changes
    only the initial guess for Eq. (8), not the LP constraints themselves.
    """
    if previous_replay is None:
        return np.tile(current_levels, (horizon, 1))

    previous_replay = np.asarray(previous_replay, float)
    if previous_replay.ndim != 2 or previous_replay.shape[1] != 2 or len(previous_replay) < 2:
        return np.tile(current_levels, (horizon, 1))

    # Expected end states for the shifted horizon. The final point is held
    # constant because the new last horizon step was not predicted previously.
    old = previous_replay
    ends = np.vstack([old[1:], old[-1]])
    if len(ends) < horizon:
        ends = np.vstack([ends, np.tile(ends[-1], (horizon - len(ends), 1))])
    ends = ends[:horizon]

    starts = np.vstack([current_levels, ends[:-1]])
    return 0.5 * (starts + ends)


def update_relaxation(alpha, error, previous_error):
    """Adapt the relaxation factor alpha in Eq. (8).

    If the LP/EPANET mismatch (Eq. 9) falls clearly, the method takes a larger
    step toward the replayed trajectory.  If mismatch grows, alpha is reduced to
    damp possible oscillation.  Bounds [0.2, 0.8] keep updates conservative.
    """
    if previous_error is None or not np.isfinite(previous_error):
        return float(alpha)
    if error < 0.90 * previous_error:
        return min(0.80, alpha + 0.10)
    if error > 1.05 * previous_error:
        return max(0.20, alpha * 0.50)
    return float(alpha)


def optimize_horizon(
    network,
    levels,
    forecast,
    prices,
    previous_state,
    iterations,
    tolerance,
    terminal_penalty,
    min_segment_s,
    initial_trial=None,
    sequence_ranking="energy",
    adaptive_relaxation=True,
):
    """Optimize one prediction horizon using iterative LP/EPANET refinement.

    This function is the core of the Jowitt-v2 controller.  One refinement
    iteration performs the following sequence:

    1. Evaluate EPANET at representative future tank levels to obtain q and P.
    2. Solve Eqs. (1)-(7) with ``linprog``.
    3. Convert optimal fractions into executable within-hour schedules.
    4. Replay the complete horizon using nonlinear EPANET hydraulics.
    5. Evaluate the trajectory error in Eq. (9).
    6. If necessary, update the hydraulic trial trajectory using Eq. (8).

    The best feasible plan encountered is retained even if the refinement loop
    reaches its maximum number of iterations before meeting the tolerance.
    """
    H = len(forecast)
    trial = (
        np.asarray(initial_trial, float).copy()
        if initial_trial is not None and np.shape(initial_trial) == (H, 2)
        else np.tile(levels, (H, 1))
    )

    # Initial damping in Eq. (8).  With adaptive refinement disabled, alpha
    # remains at the v1 value of 0.5.
    alpha = 0.50
    previous_error = None
    best = None

    for iteration in range(1, iterations + 1):
        # Step A: local hydraulic characterization.  q and P depend on head, so
        # they are recomputed at the current trial trajectory.
        net, power = network.mode_table(trial, forecast)
        # Step B: solve the linear economic scheduling problem, Eqs. (1)-(7).
        fractions, predicted = solve_lp(
            levels, forecast, prices, net, power, network, terminal_penalty
        )

        state = levels.copy()
        current = previous_state.copy()
        replay = []
        midpoints = []
        actions = []
        cost = 0.0
        total_starts = 0

        # Step C: turn the LP fractions into an executable schedule and replay
        # the entire H-hour plan in EPANET, one forecast hour at a time.
        try:
            for h in range(H):
                mode_cost = prices[h] * power[h]
                seconds, order, result = choose_sequence(
                    network,
                    state,
                    forecast[h],
                    fractions[h],
                    current,
                    min_segment_s=min_segment_s,
                    mode_cost=mode_cost,
                    ranking=sequence_ranking,
                )
                next_state, _, _, energy, starts, current, _ = result
                # Pump coefficients are most representative near the average
                # hydraulic state experienced during the hour.  These midpoint
                # levels become the target of the next Eq. (8) update.
                midpoints.append((state + next_state) / 2.0)
                state = next_state
                replay.append(state.copy())
                actions.append((seconds, tuple(order)))
                cost += prices[h] * energy.sum()
                total_starts += int(starts.sum())
        except RuntimeError:
            # No hydraulically feasible ordering was found for at least one
            # forecast hour.  Rather than abandoning the horizon immediately,
            # move cautiously toward the LP-predicted trajectory and recalibrate.
            predicted_starts = np.vstack([levels, predicted[:-1]])
            predicted_midpoints = 0.5 * (predicted_starts + predicted)
            alpha = max(0.20, alpha * 0.50)
            trial = (1.0 - alpha) * trial + alpha * predicted_midpoints
            continue

        replay = np.asarray(replay)
        midpoints = np.asarray(midpoints)
        # Eq. (9): maximum absolute difference between the linear prediction
        # and the nonlinear EPANET end-of-hour tank levels over the horizon.
        error = float(np.max(np.abs(replay - predicted)))

        terminal_shortfall = np.maximum(network.initial - state, 0.0) @ network.area
        reserve_shortfall = (
            np.maximum(network.minimum + RESERVE_M - replay, 0.0) * network.area
        ).sum()
        # Compare feasible refinement iterations with the same economic logic
        # as Eq. (7), but use *actual replayed EPANET energy and storage states*.
        # This merit is used only to retain the best feasible iteration.
        merit = (
            cost
            + terminal_penalty * terminal_shortfall
            + RESERVE_PENALTY * reserve_shortfall
        )

        if best is None or merit < best["merit"]:
            best = {
                "actions": actions,
                "replay": replay.copy(),
                "merit": float(merit),
                "error_m": error,
                "iterations": iteration,
                "starts": total_starts,
            }

        # Stop once the surrogate and nonlinear hydraulic trajectories agree
        # within the requested tolerance.  Otherwise apply Eq. (8) and repeat.
        if error <= tolerance:
            break

        if adaptive_relaxation:
            alpha = update_relaxation(alpha, error, previous_error)
        else:
            alpha = 0.50
        # Eq. (8): damped successive update of the hydraulic linearization path.
        trial = (1.0 - alpha) * trial + alpha * midpoints
        previous_error = error

    if best is None:
        raise RuntimeError("No feasible LP/hydraulic plan found")
    return best


def run(
    data_dir,
    output_dir,
    hours=48,
    horizon=12,
    seed=42,
    iterations=8,
    tolerance=0.005,
    min_pressure_m=DEFAULT_MIN_PRESSURE_M,
    min_segment_s=DEFAULT_MIN_SEGMENT_S,
    terminal_penalty=DEFAULT_TERMINAL_PENALTY,
    warm_start=True,
    sequence_ranking="energy",
    adaptive_relaxation=True,
):
    """Run the complete closed-loop experiment.

    This is the outer rolling-horizon loop described by Eq. (10).  The optimizer
    sees the nominal demand forecast, while the hydraulic system experiences a
    reproducible +/-10% multiplicative demand perturbation.  Consequently each
    new optimization starts from the state that actually resulted under the
    realized demand rather than blindly following the old forecast.
    """
    nominal, tariffs, tanks = load_data(data_dir)
    # Create one reproducible demand realization for the *executed* system.
    # The optimization itself continues to use the nominal forecast.
    rng = np.random.default_rng(seed)
    realized = nominal[np.arange(hours) % 24] * rng.uniform(0.9, 1.1, (hours, 2))

    hourly_rows = []
    segment_rows = []
    starts = np.zeros(4, dtype=int)
    runtime = np.zeros(4)
    previous_replay = None

    with tempfile.TemporaryDirectory(prefix="jowitt_v3_") as work_dir:
        network = WaterNetwork(
            data_dir / "small_well_net.inp", tanks, work_dir, min_pressure_m=min_pressure_m
        )
        state = network.initial.copy()
        previous = np.zeros(4, dtype=int)

        try:
            for hour in range(hours):
                # Eq. (10), step 1: construct the H-hour forecast window.
                # Modulo 24 repeats the daily demand/tariff pattern on day 2.
                idx = (hour + np.arange(horizon)) % 24
                initial_trial = (
                    warm_start_trial(state, previous_replay, horizon)
                    if warm_start
                    else None
                )
                # Eq. (10), step 2: optimize the whole future horizon.
                plan = optimize_horizon(
                    network,
                    state,
                    nominal[idx],
                    tariffs[idx],
                    previous,
                    iterations,
                    tolerance,
                    terminal_penalty,
                    min_segment_s,
                    initial_trial=initial_trial,
                    sequence_ranking=sequence_ranking,
                    adaptive_relaxation=adaptive_relaxation,
                )
                previous_replay = plan["replay"]
                # Eq. (10), step 3: implement ONLY the first-hour decision.
                # The rest of the horizon is a prediction and will be discarded
                #/recomputed after the actual state at the next hour is known.
                seconds, order = plan["actions"][0]
                result = simulate_sequence(
                    # Use realized demand here, intentionally different from
                    # the nominal demand used by the optimizer above.
                    network, state, realized[hour], seconds, order, previous
                )
                if result is None:
                    raise RuntimeError(f"Hydraulic failure at hour {hour}")

                next_state, minimum, _, energy_by_pump, new_starts, previous, min_pressure = result
                starts += new_starts
                runtime += seconds @ MODES / 3600.0

                energy = energy_by_pump.sum()
                tariff = tariffs[hour % 24]
                hourly_rows.append(
                    {
                        "hour": hour,
                        "tariff": tariff,
                        "cost": tariff * energy,
                        "energy_kwh": energy,
                        "t1_end_m": next_state[0],
                        "t2_end_m": next_state[1],
                        "t1_min_m": minimum[0],
                        "t2_min_m": minimum[1],
                        "min_pressure_m": min_pressure,
                        "refinement_iterations": plan["iterations"],
                        "replay_error_m": plan["error_m"],
                    }
                )

                for mode in order:
                    segment_rows.append(
                        {
                            "hour": hour,
                            "mode": int(mode),
                            "seconds": int(seconds[mode]),
                            **{pump: int(MODES[mode, j]) for j, pump in enumerate(PUMPS)},
                        }
                    )

                # Eq. (10), step 4: this nonlinear EPANET end state becomes the
                # measured/current state from which the next hour is optimized.
                state = next_state
                print(
                    f"{hour + 1:02d}/{hours}: cost={tariff * energy:.2f}, "
                    f"levels=({state[0]:.3f}, {state[1]:.3f}), "
                    f"iter={plan['iterations']}, err={1000 * plan['error_m']:.1f} mm"
                )
        finally:
            network.close()

    hourly = pd.DataFrame(hourly_rows)
    area = np.pi * tanks["diameter"].to_numpy(float) ** 2 / 4
    # Aggregate outputs useful for engineering assessment and for comparing v2
    # against v1: economics, storage safety, switching, and refinement quality.
    summary = {
        "algorithm": "jowitt_v2",
        "cost": float(hourly["cost"].sum()),
        "energy_kwh": float(hourly["energy_kwh"].sum()),
        "peak_tariff_energy_kwh": float(
            hourly.loc[hourly["tariff"] > tariffs.min(), "energy_kwh"].sum()
        ),
        "final_levels_m": state.tolist(),
        "final_storage_m3": float(state @ area),
        "min_levels_m": [
            float(hourly["t1_min_m"].min()),
            float(hourly["t2_min_m"].min()),
        ],
        "minimum_pressure_m": float(hourly["min_pressure_m"].min()),
        "starts": dict(zip(PUMPS, starts.tolist())),
        "runtime_hours": dict(zip(PUMPS, runtime.tolist())),
        "mean_refinement_iterations": float(hourly["refinement_iterations"].mean()),
        "max_refinement_iterations": int(hourly["refinement_iterations"].max()),
        "max_replay_error_m": float(hourly["replay_error_m"].max()),
        "converged_within_tolerance": int((hourly["replay_error_m"] <= tolerance).sum()),
        "settings": {
            "hours": int(hours),
            "horizon": int(horizon),
            "seed": int(seed),
            "max_iterations": int(iterations),
            "tolerance_m": float(tolerance),
            "min_pressure_m": float(min_pressure_m),
            "min_segment_s": int(min_segment_s),
            "terminal_penalty": float(terminal_penalty),
            "warm_start": bool(warm_start),
            "sequence_ranking": str(sequence_ranking),
            "adaptive_relaxation": bool(adaptive_relaxation),
        },
    }

    output_dir.mkdir(parents=True, exist_ok=True)
    hourly.to_csv(output_dir / "hourly.csv", index=False)
    pd.DataFrame(segment_rows).to_csv(output_dir / "segments.csv", index=False)
    (output_dir / "summary.json").write_text(json.dumps(summary, indent=2))
    return summary


def main():
    """Command-line entry point; all important v2 enhancements are configurable."""
    parser = argparse.ArgumentParser(description="Jowitt-style LP pump scheduler v3")
    parser.add_argument("--data-dir", type=Path, default=Path(__file__).parent / "data")
    parser.add_argument("--output-dir", type=Path, default=Path(__file__).parent / "results")
    parser.add_argument("--hours", type=int, default=48)
    parser.add_argument("--horizon", type=int, default=12)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--iterations", type=int, default=8)
    parser.add_argument("--tolerance", type=float, default=0.005)
    parser.add_argument("--min-pressure", type=float, default=DEFAULT_MIN_PRESSURE_M)
    parser.add_argument("--min-segment", type=int, default=DEFAULT_MIN_SEGMENT_S)
    parser.add_argument("--terminal-penalty", type=float, default=DEFAULT_TERMINAL_PENALTY)
    parser.add_argument("--no-warm-start", action="store_true")
    parser.add_argument(
        "--sequence-ranking", choices=["energy", "clearance"], default="energy"
    )
    parser.add_argument(
        "--fixed-relaxation", action="store_true",
        help="Use the v1 fixed 0.5 LP/EPANET relaxation instead of adaptive damping",
    )
    args = parser.parse_args()

    summary = run(
        data_dir=args.data_dir,
        output_dir=args.output_dir,
        hours=args.hours,
        horizon=args.horizon,
        seed=args.seed,
        iterations=args.iterations,
        tolerance=args.tolerance,
        min_pressure_m=args.min_pressure,
        min_segment_s=args.min_segment,
        terminal_penalty=args.terminal_penalty,
        warm_start=not args.no_warm_start,
        sequence_ranking=args.sequence_ranking,
        adaptive_relaxation=not args.fixed_relaxation,
    )
    print(json.dumps(summary, indent=2))


if __name__ == "__main__":
    main()