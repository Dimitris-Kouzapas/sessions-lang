"""
External pump control.

Example pump command:
    {hour: 2, "signal": 1}
    {hour: 11, "signal": 0}

This simulator:
    - Loads the EPANET network
    - Publishes tank levels and demands
    - Receives separate MQTT commands for each pump
    - Runs hydraulic analysis step-by-step
"""

import argparse
from pathlib import Path
import tempfile
import numpy as np

from mqtt_client import MQTTClient
import configuration as config
from water_network import WaterNetwork

def main():
    # ---------------------------------------------------------
    # parse arguments
    # ---------------------------------------------------------

    parser = argparse.ArgumentParser(description="Jowitt-style LP pump scheduler")
    parser.add_argument("--data-dir", type=Path, default=Path(__file__).parent / "data")
    #parser.add_argument("--output-dir", type=Path, default=Path(__file__).parent / "results")
    parser.add_argument("--hours", type=int, default=config.DEFAULT_HOURS)
    #parser.add_argument("--horizon", type=int, default=12)
    parser.add_argument("--seed", type=int, default=42)
    parser.add_argument("--min-pressure", type=float, default=config.DEFAULT_MIN_PRESSURE_M)

    args = parser.parse_args()

    conf, tanks = config.configuration()
    nominal = np.array(conf["nominal"])

    # ---------------------------------------------------------
    # setup mqtt client
    # ---------------------------------------------------------

    mqtt_client = MQTTClient("epyt-simulator")

    for i, p in enumerate(conf["pumps"]):
        mqtt_client.subscribe(
            f"sim/{p}",
            1,
        )

    mqtt_client.start()

    # ---------------------------------------------------------
    # setup WaterNetwork 
    # ---------------------------------------------------------

    hours = args.hours

    # Create one reproducible demand realisation for the *executed* system.
    rng = np.random.default_rng(args.seed)
    realised = nominal[np.arange(hours) % 24] * rng.uniform(0.9, 1.1, (hours, 2))

    previous_replay = None

    with tempfile.TemporaryDirectory(prefix="water_network_") as work_dir:
        network = WaterNetwork(
            args.data_dir / "small_well_net.inp", tanks, work_dir, min_pressure_m=args.min_pressure
        )
        state = network.initial.copy()
        previous = np.zeros(4, dtype=int)

        hour = 0
        try:
            while hour < hours:

                # -----------------------------------------------------
                # publish tank state
                # -----------------------------------------------------

                for i, t in enumerate(conf["tanks"]):
                    mqtt_client.publish(
                        f"sim/{t}",
                        {
                            "hour": hour,
                            "level": float(state[i])
                        }, 
                    )

                # -----------------------------------------------------
                # publish demand information
                # -----------------------------------------------------

                for i, d in enumerate(conf["demands"]): 
                    mqtt_client.publish(
                        f"sim/{d}",
                        {
                            "hour": hour,
                            "demand": float(realised[hour][i])
                        }
                    )

                secs = 0
                while secs < 3600:
                    pump_state = []
                    # -----------------------------------------------------
                    # receive schedule for pumps
                    # -----------------------------------------------------

                    for i, p in enumerate(conf["pumps"]):
                        command = mqtt_client.get(
                            f"sim/{p}",
                            True
                        )
                        pump_state.append( command["signal"] )
                        seconds = command["seconds"]
                    secs += seconds

                    np_pump_state = np.array(pump_state)
                    result = network.run_segment(state, realised[hour], np_pump_state, seconds)
                    state = result["levels"]

                    print(
                        "hour =", hour,

                        "tank1 =", float(state[0]),
                        "tank2 =", float(state[1]),

                        "pump state = ", pump_state, 

                        "| seconds =", seconds,
                    )

                hour += 1
        finally:
            network.close()

if __name__ == "__main__":
    main()