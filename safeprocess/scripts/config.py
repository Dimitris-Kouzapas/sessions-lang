"""Experiment configuration: uncertainty bounds, scenario and diagnoser settings.

Everything the diagnoser is allowed to know about the uncertainty is declared here,
before the experiment starts. The simulator draws the *true* parameters and sensor
errors inside these bounds; the diagnoser only ever sees the bounds.
"""

INP_FILE = "data/small_well_net.inp"
DT_S = 900                      # sampling period: 15 min
DT_H = DT_S / 3600.0
SIM_HOURS = 24                  # the .inp duration
SEED = 7

# ---------------------------------------------------------------------------
# Uncertainty bounds (known to the diagnoser)
# ---------------------------------------------------------------------------
UNC = dict(
    C_rel=0.10,             # Hazen-Williams C of every pipe, relative
    pump_rel=0.03,          # pump curve head (shut-off head a and curvature b scale together)
    demand_meter_rel=0.05,  # aggregated smart-meter demand, relative to the true demand
    flow_meter_rel=0.02,    # flow meters, fixed gain error, relative to the true flow
    level_abs=0.01,         # level sensors, m, step-to-step noise
    pressure_offset=0.18,   # pressure sensors, fixed calibration offset, m
    pressure_noise=0.02,    # pressure sensors, step-to-step noise, m
)

# Real consumption deviates from the demand pattern by up to this fraction.
# The diagnoser does not need this number: it uses the metered demands.
TRUE_DEMAND_VARIATION = 0.10

# ---------------------------------------------------------------------------
# Scenario
# ---------------------------------------------------------------------------
SCENARIO = dict(
    # healthy window used to cross-calibrate the pressure-based virtual levels
    calibration_window_h=(0.0, 6.0),
    # sensor failure: level transmitter of tank t1 freezes at its last value
    sensor_fault=dict(sensor="level_t1", start_h=7.0, kind="stuck"),
    # process fault: unmetered outflow (leak) from tank t1
    leak=dict(tank="t1", start_h=17.0, flow_m3h=70.0),
)

# ---------------------------------------------------------------------------
# Diagnoser settings
# ---------------------------------------------------------------------------
DIAG = dict(
    lam=0.9,                 # estimator pole (error time constant about 2.4 h)
    persistence=2,           # consecutive steps a static residual must exceed its bound
    sensor_window=8,         # steps within which a head-residual alarm and a tank alarm must coincide
    signature_window=4,      # steps of static-residual history used for the fault signature
    fie_mu0=1.0,             # learning rate of the isolation estimator (0 < mu0 < 2)
    fie_c=1.0,               # normalisation constant of the learning law
    leak_set=(0.0, 200.0),   # admissible leak flows, m3/h
    lipschitz_margin=1.5,    # safety factor on the numerically computed Lipschitz constants
)
