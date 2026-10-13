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

import argparse
from pathlib import Path
import tempfile
import itertools

# Numerical/data libraries.
# NumPy stores trajectories and matrices, pandas reads/writes tabular inputs, and
# SciPy's linprog solves the linear optimization problem defined by Eqs. (1)-(7).
import numpy as np
from scipy.optimize import linprog

from mqtt_client import MQTTClient
import configuration as config
import water_network as wn

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
	c[r] = config.RESERVE_PENALTY

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
		np.tile(-(network.minimum + config.RESERVE_M) * network.area, H),
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
			(network.minimum[tank] + config.PHYSICAL_MARGIN_M) * network.area[tank],
			(network.maximum[tank] - config.RESERVE_M) * network.area[tank],
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
		distances = np.sum(wn.MODES[receivers] != wn.MODES[m], axis=1)
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
		pump_state = wn.MODES[mode]
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
	min_segment_s=config.DEFAULT_MIN_SEGMENT_S,
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

	This function is the core of the Jowitt-v3 controller.  One refinement
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
			np.maximum(network.minimum + config.RESERVE_M - replay, 0.0) * network.area
		).sum()
		# Compare feasible refinement iterations with the same economic logic
		# as Eq. (7), but use *actual replayed EPANET energy and storage states*.
		# This merit is used only to retain the best feasible iteration.
		merit = (
			cost
			+ terminal_penalty * terminal_shortfall
			+ config.RESERVE_PENALTY * reserve_shortfall
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

def main():

	# ---------------------------------------------------------
	# parse arguments
	# ---------------------------------------------------------

	parser = argparse.ArgumentParser(description="Jowitt-style LP pump scheduler")
	parser.add_argument("--data-dir", type=Path, default=Path(__file__).parent / "data")
	#parser.add_argument("--output-dir", type=Path, default=Path(__file__).parent / "results")
	parser.add_argument("--hours", type=int, default=config.DEFAULT_HOURS)
	parser.add_argument("--horizon", type=int, default=12)
	parser.add_argument("--iterations", type=int, default=8)
	parser.add_argument("--tolerance", type=float, default=0.005)
	parser.add_argument("--min-pressure", type=float, default=config.DEFAULT_MIN_PRESSURE_M)
	parser.add_argument("--min-segment", type=int, default=config.DEFAULT_MIN_SEGMENT_S)
	parser.add_argument("--terminal-penalty", type=float, default=config.DEFAULT_TERMINAL_PENALTY)
	parser.add_argument("--no-warm-start", action="store_true")
	parser.add_argument(
		"--sequence-ranking", choices=["energy", "clearance"], default="energy"
	)
	parser.add_argument(
		"--fixed-relaxation", action="store_true",
		help="Use the v1 fixed 0.5 LP/EPANET relaxation instead of adaptive damping",
	)

	args = parser.parse_args()
	conf, tanks = config.configuration()

	# ---------------------------------------------------------
	# setup mqtt client
	# ---------------------------------------------------------

	mqtt_client = MQTTClient("optimiser")

	mqtt_client.subscribe(
		"sim/optimise",
		1,
	)

	mqtt_client.start()

	# ---------------------------------------------------------
	# setup WaterNetwork 
	# ---------------------------------------------------------

	"""Run the complete closed-loop experiment.

	This is the outer rolling-horizon loop described by Eq. (10).  The optimizer
	sees the nominal demand forecast, while the hydraulic system experiences a
	reproducible +/-10% multiplicative demand perturbation.  Consequently each
	new optimization starts from the state that actually resulted under the
	realized demand rather than blindly following the old forecast.
	"""

	warm_start = not args.no_warm_start
	hours = args.hours

	previous_replay = None

	with tempfile.TemporaryDirectory(prefix="jowitt_optimiser_") as work_dir:
		network = wn.WaterNetwork(
			args.data_dir / "small_well_net.inp", tanks, work_dir, min_pressure_m=args.min_pressure
		)

		previous = np.zeros(4, dtype=int)

		hour = 0

		try:
			while hour < hours - 1:
				request = 	mqtt_client.get(
								"sim/optimise",
								True,
							)

				hour = request["hour"]
				levels = request["current_levels"]
				forecast = request["demand_forecast"]
				prices = request["prices"]

				np_levels = np.array(levels)
				np_forecast = np.array(forecast)
				np_prices = np.array(prices)

				initial_trial = (
					warm_start_trial(np_levels, previous_replay, args.horizon)
					if warm_start
					else None
				)
				# Eq. (10), step 2: optimize the whole future horizon.
				plan = optimize_horizon(
					network,
					np_levels,
					np_forecast,
					np_prices,
					previous,
					args.iterations,
					args.tolerance,
					args.terminal_penalty,
					args.min_segment,
					initial_trial=initial_trial,
					sequence_ranking=args.sequence_ranking,
					adaptive_relaxation=not args.fixed_relaxation,
				)

				previous_replay = plan["replay"]

				# Eq. (10), step 3: implement ONLY the first-hour decision.
				# The rest of the horizon is a prediction and will be discarded
				#/recomputed after the actual state at the next hour is known.
				seconds, order = plan["actions"][0]

				order_list = []
				seconds_list = []
				modes = []

				for mode in order:
					m = int(mode)
					seconds_list.append(int(seconds[m]))
					modes = wn.MODES[m].tolist()
					pump_status = {}
					pump_status["pump1"] = modes[0]
					pump_status["pump2"] = modes[1]
					pump_status["pump3"] = modes[2]
					pump_status["booster"] = modes[3]
					order_list.append(pump_status)

				previous = wn.MODES[int(order[-1])].astype(int).copy()

				prediction = {
					"seconds": seconds_list,
					"order": order_list,
				}

				print("hour = ", hour, " | prediction = ", prediction)

				mqtt_client.publish(
					"sim/prediction",
					prediction,
				)
		finally:
			network.close()



if __name__ == "__main__":
	main()