#!/usr/bin/env python3

import itertools
import shutil
import tempfile
from pathlib import Path


# NumPy stores trajectories and matrices, pandas reads/writes tabular inputs, and
import numpy as np
import pandas as pd

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
TANKS = ["tank1", "tank2"]
DEMAND_NODES = ["12", "11"]

# All binary ON/OFF combinations are enumerated once.
# WELL_MODES contains 2^3 = 8 combinations of the three well pumps.
# MODES contains 2^4 = 16 combinations of all pumps, including the booster.
# A row such as [1, 0, 1, 1] means pumps 1, 3 and the booster are ON.
WELL_MODES = np.array(list(itertools.product([0, 1], repeat=3)), dtype=int)
MODES = np.array(list(itertools.product([0, 1], repeat=4)), dtype=int)

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

