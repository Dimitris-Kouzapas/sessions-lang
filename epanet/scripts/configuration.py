import numpy as np
import pandas as pd
from pathlib import Path

# Preferred storage reserve above the tank minimum [m].  This is soft in the LP
# through Eq. (5), while PHYSICAL_MARGIN_M contributes to the hard lower bound.
RESERVE_M = 0.30
PHYSICAL_MARGIN_M = 0.05
RESERVE_PENALTY = 1000.0
DEFAULT_TERMINAL_PENALTY = 10.0
DEFAULT_MIN_PRESSURE_M = 20.0
DEFAULT_MIN_SEGMENT_S = 300

DEFAULT_HOURS = 48

import json

def configuration(file="data/configuration.json"):
    with open(file, "r", encoding="utf-8") as f:
        config = json.load(f)
    tanks = pd.DataFrame.from_dict(
        {
            name: config[name]
            for name in config["tanks"]
        },
        orient="index",
    )
    return config, tanks
