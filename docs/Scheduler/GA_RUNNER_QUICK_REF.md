# Task 4.6: GA Runner - Quick Reference

## Overview
Orchestrates complete genetic algorithm evolution to optimize scheduling parameters.

## Key Functions

### RunGAEpoch
```go
func RunGAEpoch(ctx context.Context, historyDB *db.HistoryDB, config GAConfig, paramsOutputPath string) error
```
**Purpose**: Main entry point for GA training cycle  
**Steps**: Fetch data → Train Theta → Initialize population → Evolve → Build matrices → Save params

### initializePopulation
```go
func initializePopulation(config GAConfig, trainedTheta scheduler.Theta) Population
```
**Purpose**: Create initial population (1 trained + N-1 random chromosomes)

### evaluatePopulation
```go
func evaluatePopulation(pop Population, history []db.TaskHistory, workerStats []db.WorkerStats, config GAConfig) Population
```
**Purpose**: Compute fitness for all chromosomes

### tournamentSelection
```go
func tournamentSelection(pop Population, tournamentSize int) Chromosome
```
**Purpose**: Select high-fitness parent (pick k random, return best)

### crossover
```go
func crossover(parent1, parent2 Chromosome, crossoverRate float64) (Chromosome, Chromosome)
```
**Purpose**: Uniform crossover (50% chance per gene from each parent)

### mutate
```go
func mutate(c Chromosome, mutationRate float64) Chromosome
```
**Purpose**: Gaussian mutation with bounds clipping

## Configuration

```go
config := aod.GetDefaultGAConfig()
// PopulationSize: 20
// Generations: 10
// MutationRate: 0.1
// CrossoverRate: 0.7
// ElitismCount: 2
// TournamentSize: 3
```

## Usage

```go
// Run one GA epoch
err := aod.RunGAEpoch(ctx, historyDB, config, "config/ga_output.json")

// Start periodic training (in main.go)
go func() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        aod.RunGAEpoch(context.Background(), historyDB, config, "config/ga_output.json")
    }
}()
```

## Output (ga_output.json)

```json
{
  "theta": {"theta1": 0.15, "theta2": 0.10, "theta3": 0.28, "theta4": 0.21},
  "risk": {"alpha": 12.5, "beta": 1.2},
  "affinity_weights": {"a1": 1.2, "a2": 2.3, "a3": 0.6},
  "penalty_weights": {"g1": 2.1, "g2": 1.1, "g3": 0.55},
  "affinity_matrix": {
    "cpu-light": {"worker1": 1.5, "worker2": -0.3},
    "gpu-inference": {"worker1": 0.2, "worker2": 1.8}
  },
  "penalty_vector": {
    "worker1": 0.3,
    "worker2": 0.8
  }
}
```

## Algorithm Flow

```
1. Fetch History (last 24h)
2. Train Theta (linear regression)
3. Initialize Population (20 chromosomes)
4. For each generation (10 iterations):
   - Evaluate fitness (all chromosomes)
   - Sort by fitness
   - Keep top 2 (elitism)
   - Fill remaining 18:
     * Tournament selection (k=3)
     * Crossover (70% rate)
     * Mutate (10% rate per gene)
5. Extract best chromosome
6. Build affinity matrix (best weights)
7. Build penalty vector (best weights)
8. Save to JSON
```

## Parameter Ranges

| Parameter | Min | Max | Default | Random Range |
|-----------|-----|-----|---------|--------------|
| Theta1-2  | 0.0 | 2.0 | 0.1     | [0.0, 0.5]   |
| Theta3    | 0.0 | 2.0 | 0.3     | [0.0, 0.8]   |
| Theta4    | 0.0 | 2.0 | 0.2     | [0.0, 0.6]   |
| Alpha     | 0.1 | 100 | 10.0    | [5.0, 20.0]  |
| Beta      | 0.1 | 100 | 1.0     | [0.5, 2.5]   |
| A1        | 0.1 | 10  | 1.0     | [0.5, 2.5]   |
| A2        | 0.1 | 10  | 2.0     | [1.0, 4.0]   |
| A3        | 0.0 | 10  | 0.5     | [0.0, 1.5]   |
| G1        | 0.0 | 10  | 2.0     | [1.0, 4.0]   |
| G2        | 0.0 | 10  | 1.0     | [0.5, 2.5]   |
| G3        | 0.0 | 10  | 0.5     | [0.0, 1.5]   |

## Genetic Operators

### Tournament Selection (k=3)
- Pick 3 random chromosomes
- Return the one with highest fitness
- Selection pressure: moderate

### Uniform Crossover (rate=0.7)
- For each gene: 50% from parent1, 50% from parent2
- Produces 2 offspring per couple
- Better diversity than single-point

### Gaussian Mutation (rate=0.1)
- Add N(0, σ²) noise to each gene
- σ = {0.1 for Theta, 2.0 for Alpha, 0.2 for weights}
- Clip to valid bounds after mutation

### Elitism (count=2)
- Copy top 2 chromosomes unchanged
- Guarantees monotonic fitness improvement

## Performance

| Metric | Value |
|--------|-------|
| Runtime | ~0.5-2.0s |
| Memory | ~10-50 MB |
| Complexity | O(G × N × M) |
| Scalability | Linear with history size |

Where:
- G = Generations (10)
- N = PopulationSize (20)
- M = TaskHistory size (~100-1000)

## Testing

14 tests covering:
- ✅ Population initialization
- ✅ Fitness evaluation
- ✅ Tournament selection (+ edge cases)
- ✅ Crossover (high and zero rates)
- ✅ Mutation (bounds and validity)
- ✅ Parameter saving/loading
- ✅ Default and random helpers

Run tests:
```bash
cd master
go test ./internal/aod -v -run TestGA -count=1
```

## Troubleshooting

### Low Convergence
- Increase PopulationSize (20 → 50)
- Increase Generations (10 → 20)
- Tune fitness weights

### Slow Runtime
- Decrease PopulationSize (20 → 10)
- Decrease Generations (10 → 5)
- Run less frequently (60s → 120s)

### Invalid Chromosomes
- Check mutation bounds
- Verify crossover logic
- Increase random diversity

## Integration Points

| Component | Connection |
|-----------|-----------|
| Task 4.2 | TrainTheta() for linear regression |
| Task 4.3 | BuildAffinityMatrix() for task-worker scores |
| Task 4.4 | BuildPenaltyVector() for worker reliability |
| Task 4.5 | ComputeFitness() for chromosome evaluation |
| Task 4.7 | Hot-reload in RTS scheduler (every 30s) |

## Next Steps

1. ✅ Task 4.6 complete (GA Runner implemented)
2. → Task 4.7: Integrate AOD into Master
   - Add GA epoch ticker in main.go
   - Start params reloader in RTS scheduler
   - Test end-to-end evolution
