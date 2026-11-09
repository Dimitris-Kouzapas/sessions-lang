import pandas as pd
import numpy as np
import matplotlib.pyplot as plt

# Load data
df = pd.read_csv("results/data.csv")

time = df["timestep"] * 0.25  # 15-minute intervals
residual = df["eps_q1"].abs()
threshold = df["threshold_q1"]
alarm = df["alarm_q1"].astype(str).str.lower() == "true"

fig, ax = plt.subplots(figsize=(11, 5))

# Residual and adaptive threshold
ax.plot(time, residual.replace(0, np.nan),
        color="tab:blue", linewidth=1.8,
        label=r"$|\varepsilon_{Q1}|$")

ax.plot(time, threshold,
        color="tab:orange", linestyle="--", linewidth=1.8,
        label=r"$T_{Q1}$")

# Fault detections as red dots
ax.scatter(time[alarm], residual[alarm],
           color="red", s=30, zorder=5,
           label="Fault detection")

# Experimental events
ax.axvline(7.0, color="gray", linestyle=":", linewidth=1.2)
ax.axvline(7.25, color="green", linestyle=":", linewidth=1.2)
ax.axvline(17.0, color="gray", linestyle=":", linewidth=1.2)
ax.axvline(18.5, color="red", linestyle=":", linewidth=1.2)

# Reconfigured operating period
ax.axvspan(7.25, 24, color="gray", alpha=0.08)

# Event annotations
ax.annotate("Sensor failure\n07:00",
            xy=(7.0, 40.668),
            xytext=(4.5, 130),
            arrowprops=dict(arrowstyle="->"),
            fontsize=9)

ax.annotate("Reconfiguration\n07:15",
            xy=(7.25, 19.635),
            xytext=(8.5, 65),
            arrowprops=dict(arrowstyle="->"),
            fontsize=9)

ax.annotate("Leak introduced\n17:00",
            xy=(17.0, 17.155),
            xytext=(14.0, 105),
            arrowprops=dict(arrowstyle="->"),
            fontsize=9)

# ax.annotate("Sensor failure\n07:00",
#             xy=(7.0, 40.668),
#             xytext=(3.8, 90),
#             arrowprops=dict(arrowstyle="->"),
#             fontsize=9)

# ax.annotate("Reconfiguration\n07:15",
#             xy=(7.25, 19.635),
#             xytext=(8.3, 8),
#             arrowprops=dict(arrowstyle="->"),
#             fontsize=9)

# ax.annotate("Leak introduced\n17:00",
#             xy=(17.0, 17.155),
#             xytext=(14, 7),
#             arrowprops=dict(arrowstyle="->"),
#             fontsize=9)

ax.annotate("Leak detected\n18:30",
            xy=(18.5, 71.961),
            xytext=(19.3, 22),
            arrowprops=dict(arrowstyle="->"),
            fontsize=9)

ax.set_yscale("log")
ax.set_xlim(0, 24)
ax.set_xticks(np.arange(0, 25, 2))
ax.set_xlabel("Time (hours)")
ax.set_ylabel(r"Residual / threshold (m$^3$)")
ax.set_title("Tank 1 fault-detection residual and adaptive threshold")

ax.grid(True, which="both", alpha=0.25)
ax.legend(loc="upper left")

plt.tight_layout()
plt.savefig("results/tank1_detection.pdf", bbox_inches="tight")
plt.savefig("results/tank1_detection.png", dpi=300, bbox_inches="tight")
plt.show()