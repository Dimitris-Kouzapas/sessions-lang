# Running the Digital Twin Control Application

This guide explains how to compile the Go application, install the required Python packages, start the Mosquitto MQTT broker on Windows, and run the Go and Python programs.

## Prerequisites

Make sure the following are installed and available:

- Go
- Python
- Mosquitto MQTT Broker

## 1. Build the Go Application

From the project root directory, compile the Go program:

```bat
go build -o sessions.exe main.go
```

This generates:

```text
sessions.exe
```

## 2. Install the Python Requirements

From the project root directory, install the Python packages listed in `epanet/scripts/specifications.txt`:

```bat
python -m pip install -r epanet/scripts/specifications.txt
```

Alternatively, after navigating to the scripts directory:

```bat
cd epanet/scripts/
python -m pip install -r specifications.txt
```

This step only needs to be repeated when the Python dependencies change.

## 3. Start Mosquitto

Mosquitto must be running before starting the Go application and the Python scripts.

### Option A — Mosquitto is installed as a Windows service

First, check whether the Mosquitto service exists:

```bat
sc query mosquitto
```

If the service is installed, start it from an **Administrator Command Prompt**:

```bat
net start mosquitto
```

Alternatively, from **PowerShell as Administrator**:

```powershell
Start-Service mosquitto
```

You can verify its status with:

```powershell
Get-Service mosquitto
```

The status should be:

```text
Running
```

### Option B — Mosquitto is not installed as a Windows service

If `sc query mosquitto` reports that the service does not exist, start the broker directly.

From Command Prompt:

```bat
"C:\Program Files\mosquitto\mosquitto.exe" -v
```

Or, if you are already inside the Mosquitto installation directory:

```bat
cd "C:\Program Files\mosquitto"
mosquitto.exe -v
```

Keep this terminal open while using the application.

When Mosquitto starts successfully, its output should indicate that the broker is running and listening for MQTT connections, normally on port `1883`.

> If both the Go application and the Python scripts are running on the same computer and connect to `localhost`, Mosquitto's default local-only mode is normally sufficient.

## 4. Open Three Terminals

After Mosquitto is running, open three additional terminal windows.

The Go application and the two Python scripts need to run at the same time.

## 5. Start the Go Application

In **Terminal 1**, from the project root directory, run:

```bat
sessions.exe epanet/main.sess
```

Keep this terminal running.

## 6. Start the Optimiser Python Script

In **Terminal 2**, navigate to the EPANET scripts directory:

```bat
cd epanet/scripts/
```

Then run:

```bat
python optimiser.py
```

Keep this terminal running.

## 7. Start the Simulator Python Script

In **Terminal 3**, navigate to the EPANET scripts directory:

```bat
cd epanet/scripts/
```

Then run:

```bat
python epa.py
```

Keep this terminal running.

## Execution Order

Run the components in this order:

1. Compile the Go application:

   ```bat
   go build -o sessions.exe main.go
   ```

2. Install the Python requirements:

   ```bat
   python -m pip install -r epanet/scripts/specifications.txt
   ```

3. Start the Mosquitto MQTT broker.

4. In Terminal 1, start the Go application:

   ```bat
   sessions.exe epanet/main.sess
   ```

5. In Terminal 2, start the optimiser:

   ```bat
   cd epanet/scripts/
   python optimiser.py
   ```

6. In Terminal 3, start the simulator:

   ```bat
   cd epanet/scripts/
   python epa.py
   ```

Keep Mosquitto and all three application terminals running while the digital twin control application is in use.
