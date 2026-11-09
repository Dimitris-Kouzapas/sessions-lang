# Running the Fault Detection Deployment Scenario for the small_well_net network

This guide explains how to compile the Go application, install the required Python packages, and run the Python simulation alongside the Go application on Windows.

## Prerequisites

Make sure the following are installed and available:

- Go
- Python
- Required Python dependencies

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

Install the Python packages required by the simulation.

If a `requirements.txt` file is provided, run the following command from the project root directory:

```bat
python -m pip install -r requirements.txt
```

This step only needs to be repeated when the Python dependencies change.

## 3. Open Two Terminals

Open two terminal windows.

The Python simulation and the Go application need to run at the same time.

**Important:** Start the Python simulation before starting the Go application.

## 4. Start the Python Simulation

In **Terminal 1**, from the project root directory, navigate to the simulation scripts directory:

```bat
cd safeprocess/scripts/
```

Then run:

```bat
python simulation.py
```

Keep this terminal running.

## 5. Start the Go Application

In **Terminal 2**, from the project root directory, run:

```bat
sessions.exe safeprocess/main.sess
```

Keep this terminal running.

## Execution Order

Run the components in this order:

1. Compile the Go application:

   ```bat
   go build -o sessions.exe main.go
   ```

2. Install the Python requirements (if applicable):

   ```bat
   python -m pip install -r requirements.txt
   ```

3. In Terminal 1, start the Python simulation:

   ```bat
   cd safeprocess/scripts/
   python simulation.py
   ```

4. In Terminal 2, start the Go application:

   ```bat
   sessions.exe safeprocess/main.sess
   ```

Keep both terminals running while the use case scenario is running.

## 6. Results and Visualisation

The results generated during the fault detection deployment scenario are stored in:

```text
safeprocess/scripts/results
```

### Optional — Generate Plots

To visualise the results, navigate to the scripts directory from the project root:

```bat
cd safeprocess/scripts/
```

Then run the plotting script:

```bat
python plot.py
```

This step is optional and can be performed after the simulation has completed.