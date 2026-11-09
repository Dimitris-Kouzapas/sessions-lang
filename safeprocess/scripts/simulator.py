"""True system: EPANET (through EPyT) with perturbed parameters, real demands,
sensors with bounded errors, a sensor failure and a leak.

The simulator draws the true parameters and sensor errors *inside* the bounds of
config.UNC; the diagnoser never sees these draws.
"""
import contextlib
import io
import warnings

import copy
import config

from pathlib import Path

from mqtt_client import MQTTClient 

import numpy as np
from epyt import epanet

#from hydraulics import PIPES, PUMPS

FLOW_LINKS = ["2", "4", "5", "7", "8", "9", "13", "pump1", "pump2", "pump3", "booster"]
PRESSURE_NODES = ["1", "4", "5", "8", "9", "10", "11", "12"]

PIPES = {  # id: (length m, diameter mm, C)
    "2": (2982.91, 200, 120), "4": (2182.91, 200, 120), "5": (2078.63, 250, 120),
    "7": (820.51, 250, 100), "8": (1213.68, 250, 100), "9": (1675.21, 250, 100),
    "13": (878.63, 250, 100),
}
PUMPS = {"pump1": (180, 120), "pump2": (150, 140), "pump3": (110, 110), "booster": (60, 90)}

class TrueNetwork:
    """EPANET model of the real network, stepped every dt seconds."""

    def __init__(self, inp, unc, scenario, dt_s, demand_variation, rng):
        self.dt, self.rng, self.scn = dt_s, rng, scenario
        self.demand_variation = demand_variation
        with contextlib.redirect_stdout(io.StringIO()):
            self.d = epanet(str(inp))
        warnings.filterwarnings("ignore", message=".*negative pressures.*")
        d = self.d
        d.setTimeHydraulicStep(dt_s)
        d.setTimeReportingStep(dt_s)

        # true pipe roughness, drawn inside +-C_rel
        self.C_true = {}
        for pid, (_, _, C) in PIPES.items():
            self.C_true[pid] = C * (1 + rng.uniform(-unc["C_rel"], unc["C_rel"]))
            d.setLinkRoughnessCoeff(d.getLinkIndex(pid), self.C_true[pid])

        # true pump curves: head of the single design point scaled inside +-pump_rel
        self.pump_true = {}
        curve_idx = dict(zip(d.getLinkPumpNameID(), d.getLinkPumpHCurve()))
        for pid, (qd, hd) in PUMPS.items():
            s = 1 + rng.uniform(-unc["pump_rel"], unc["pump_rel"])
            self.pump_true[pid] = s
            d.setCurve(int(curve_idx[pid]), [[qd, hd * s]])

        # leak: a short pipe from the tank to a junction whose demand is the leak flow
        self.leak_node = None
        if scenario["leak"] != None:
            tank = scenario["leak"]["tank"]
            pc = d.addPattern("const_leak", [1.0])
            self.leak_node = d.addNodeJunction("LEAK_" + tank, [4017.0, 7700.0], 0.0, 0.0)
            d.setNodeDemandPatternIndex(self.leak_node, pc)
            d.addLinkPipe("leak_pipe", tank, "LEAK_" + tank, 1.0, 150.0, 130.0)

        self.nodes = {n: i for i, n in enumerate(d.getNodeNameID())}
        self.links = {l: i for i, l in enumerate(d.getLinkNameID())}
        self.base = {"11": 90.0, "12": 300.0}
        self.v = {"11": 0.0, "12": 0.0}

    def _set_true_demands(self, t_h):
        """Real consumption: the pattern times (1 + v), v a slow bounded random process."""
        for n in self.base:
            self.v[n] = np.clip(0.9 * self.v[n] + self.rng.uniform(-0.03, 0.03),
                                -self.demand_variation, self.demand_variation)
            self.d.setNodeBaseDemands(self.nodes[n] + 1, self.base[n] * (1 + self.v[n]))
        if self.scn["leak"] != None:
            lk = self.scn["leak"]
            self.d.setNodeBaseDemands(self.leak_node, lk["flow_m3h"] if t_h >= lk["start_h"] else 0.0)

    def run(self, n_steps):
        """Generator of the true state at t = 0, dt, ..., n_steps*dt."""
        d = self.d
        d.openHydraulicAnalysis()
        d.initializeHydraulicAnalysis(0)
        k = 0
        self._set_true_demands(0.0)
        while True:
            t = d.runHydraulicAnalysis()
            if t % self.dt == 0:
                #H = d.getNodeHydraulicHead()
                P = d.getNodePressure()
                Q = d.getLinkFlows()
                D = d.getNodeActualDemand()
                S = d.getLinkStatus()
                L, N = self.links, self.nodes
                state = dict(
                    k=k, t_h=t / 3600.0,
                    #h_t1=H[N["t1"]], h_t2=H[N["t2"]],
                    level_t1=P[N["t1"]], level_t2=P[N["t2"]],
                    d11=D[N["11"]], d12=D[N["12"]], leak=(Q[L["leak_pipe"]] if self.leak_node is not None else 0.0),
                    **{"q_" + l: Q[L[l]] for l in FLOW_LINKS},
                    #**{"H_" + n: H[N[n]] for n in PRESSURE_NODES},
                    **{"P_" + n: P[N[n]] for n in PRESSURE_NODES},
                    **{"s_" + p: int(S[L[p]] > 0) for p in PUMPS},
                )
                yield state
                k += 1
                if k > n_steps:
                    break
                self._set_true_demands(k * self.dt / 3600.0)
            if d.nextHydraulicAnalysisStep() <= 0:
                break
        d.closeHydraulicAnalysis()


class Sensors:
    """Sensor models with errors drawn inside the bounds, plus the scenario's sensor failure."""

    def __init__(self, unc, scenario, rng):
        self.unc, self.rng = unc, rng
        f = unc["flow_meter_rel"]
        self.gain = {l: rng.uniform(-f, f) for l in FLOW_LINKS}
        po = unc["pressure_offset"]
        self.offset = {n: rng.uniform(-po, po) for n in PRESSURE_NODES}
        self.fault = scenario["sensor_fault"]
        self.frozen = None
        self.last = {}

    def measure(self, s):
        u, rng = self.unc, self.rng
        m = dict(k=s["k"], t_h=s["t_h"])
        for tank in ("t1", "t2"):
            #m["level_" + tank] = s["h_" + tank] + rng.uniform(-u["level_abs"], u["level_abs"])
            m["level_" + tank] = s["level_" + tank] + rng.uniform(-u["level_abs"], u["level_abs"])
        for l in FLOW_LINKS:
            m["q_" + l] = s["q_" + l] * (1 + self.gain[l])
        for n in PRESSURE_NODES:
            #m["H_" + n] = s["H_" + n] + self.offset[n] + rng.uniform(-u["pressure_noise"], u["pressure_noise"])
            m["P_" + n] = s["P_" + n] + self.offset[n] + rng.uniform(-u["pressure_noise"], u["pressure_noise"])
        e = u["demand_meter_rel"]
        m["d11"] = s["d11"] * (1 + rng.uniform(-e, e))
        m["d12"] = s["d12"] * (1 + rng.uniform(-e, e))
        for p in PUMPS:
            m["s_" + p] = s["s_" + p]

        # sensor failure
        f = self.fault
        if f and s["t_h"] >= f["start_h"]:
            if f["kind"] == "stuck":
                if self.frozen is None:
                    self.frozen = self.last.get(f["sensor"], m[f["sensor"]])
                m[f["sensor"]] = self.frozen
            elif f["kind"] == "lost":
                m[f["sensor"]] = np.nan
        self.last = m
        return m




def run(seed=config.SEED, scenario=None, unc=None, diag=None, verbose=True):
    scenario = copy.deepcopy(
        scenario if scenario is not None else config.SCENARIO
    )

    # Disable sensor faults for now
    # scenario["sensor_fault"] = None
    # Disable sensor leaks for now
    # scenario["leak"] = None

    unc = unc if unc is not None else config.UNC
    rng = np.random.default_rng(seed)

    inp_path = Path(__file__).parent / config.INP_FILE

    net = TrueNetwork(
        inp_path,
        unc,
        scenario,
        config.DT_S,
        config.TRUE_DEMAND_VARIATION,
        rng
    )

    sensors = Sensors(unc, scenario, rng)
    n_steps = int(config.SIM_HOURS * 3600 / config.DT_S)

    mqtt_client = MQTTClient()
    mqtt_client.subscribe("sim/sync/request", qos=1)
    mqtt_client.start()

    try:
        for state in net.run(n_steps):
            m = sensors.measure(state)

            request = mqtt_client.get(
                "sim/sync/request",
                block=True,
            )

            def publish(topic, value):
                mqtt_client.publish(
                    topic,
                    {
                        "timestep": int(m["k"]),
                        "value": float(value),
                    },
                    qos=1,
                )

            # Node hydraulic heads
            for n in PRESSURE_NODES:
                publish(f"sim/node/{n}", m[f"P_{n}"])

            # Tank levels
            for tank in ("t1", "t2"):
                publish(f"sim/node/{tank}", m[f"level_{tank}"])

            # Node demands
            for n in ("11", "12"):
                publish(f"sim/node/{n}/demand", m[f"d{n}"])

            # Link flows
            for link in FLOW_LINKS:
                publish(f"sim/link/{link}", m[f"q_{link}"])

            # Pump operating states
            for pump in PUMPS:
                publish(f"sim/pump/{pump}", m[f"s_{pump}"])

    finally:
        mqtt_client.stop()



# def run(seed=config.SEED, scenario=None, unc=None, diag=None, verbose=True):
#     scenario = copy.deepcopy(scenario or config.SCENARIO)
#     unc = unc or config.UNC
#     rng = np.random.default_rng(seed)
#     net = TrueNetwork(config.INP_FILE, unc, scenario, config.DT_S, config.TRUE_DEMAND_VARIATION, rng)
#     sensors = Sensors(unc, scenario, rng)
#     n_steps = int(config.SIM_HOURS * 3600 / config.DT_S)

#     rows = []
#     for state in net.run(n_steps):
#         m = sensors.measure(state)

        # I want to read and publish the sensors to MQTT


if __name__ == "__main__":
    run(config.SEED)
    