import json
import time
import random

print("=" * 50)
print("CloudAI Sample Task - Starting Execution")
print("=" * 50)

# Simulate some computation
print("\n[Step 1/3] Initializing task environment...")
time.sleep(1)

print("[Step 2/3] Processing data...")
data_points = random.randint(50, 150)
print(f"  → Processing {data_points} data points...")

total = 0
for i in range(data_points):
    total += random.randint(1, 100)
    if i % 20 == 0:
        print(f"  → Progress: {i}/{data_points}")
    time.sleep(1)

print(f"  → Completed: {data_points}/{data_points}")

print("[Step 3/3] Generating results...")
result = {
    "task_status": "completed",
    "data_points_processed": data_points,
    "computation_result": total,
    "average": total / data_points,
    "execution_time": "approximately 10 seconds",
    "timestamp": time.time()
}

# Save result to output file
with open("/output/result.json", "w") as f:
    json.dump(result, f, indent=2)

print("\n" + "=" * 50)
print("✓ Task completed successfully!")
print(f"✓ Result saved to /output/result.json")
print("=" * 50)
print("\nResult Summary:")
print(json.dumps(result, indent=2))
