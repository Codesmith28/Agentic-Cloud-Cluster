#!/usr/bin/env python3
"""
Workload Type: gpu-training
Description: Simulated hyperparameter tuning with multiple trials
Resource Requirements: High CPU, high memory, high GPU
Output: hyperparameter_results.csv (generated file)
"""

import datetime
import random
import csv
import time

def train_with_hyperparameters(trial, learning_rate, batch_size, dropout):
    """Simulate training with specific hyperparameters"""
    print(f"\n  Trial {trial}:")
    print(f"    LR={learning_rate:.2e}, Batch={batch_size}, Dropout={dropout:.2f}")
    
    # Simulate training
    epochs = 5
    final_loss = random.uniform(0.1, 1.0)
    final_accuracy = random.uniform(0.7, 0.95)
    
    # Better hyperparameters tend to give better results (simulated)
    # Optimal: LR~1e-3, Batch~64, Dropout~0.3
    lr_factor = 1.0 / (1 + abs(learning_rate - 1e-3) * 1000)
    batch_factor = 1.0 / (1 + abs(batch_size - 64) / 32)
    dropout_factor = 1.0 / (1 + abs(dropout - 0.3) * 2)
    
    combined_factor = (lr_factor + batch_factor + dropout_factor) / 3
    
    final_loss = final_loss * (1 - combined_factor * 0.5)
    final_accuracy = final_accuracy * (0.8 + combined_factor * 0.2)
    
    for epoch in range(1, epochs + 1):
        time.sleep(0.5)
        loss = final_loss * (1.5 - epoch / epochs * 0.5)
        acc = final_accuracy * (0.7 + epoch / epochs * 0.3)
        print(f"      Epoch {epoch}/{epochs}: Loss={loss:.4f}, Acc={acc:.4f}")
    
    training_time = epochs * 0.5
    
    return final_loss, final_accuracy, training_time

def run_hyperparameter_tuning():
    print(f"[{datetime.datetime.now()}] Starting Hyperparameter Tuning (GPU Training)...")
    print("🖥️  GPU Training Mode - Long Running Task")
    
    # Define hyperparameter search space
    learning_rates = [1e-4, 5e-4, 1e-3, 5e-3, 1e-2]
    batch_sizes = [32, 64, 128]
    dropout_rates = [0.1, 0.3, 0.5]
    
    num_trials = 15
    
    print(f"\n✓ Model: ResNet-18")
    print(f"✓ Search Space:")
    print(f"    Learning Rates: {learning_rates}")
    print(f"    Batch Sizes: {batch_sizes}")
    print(f"    Dropout Rates: {dropout_rates}")
    print(f"✓ Random Search Trials: {num_trials}")
    print(f"✓ Epochs per trial: 5")
    
    results = []
    
    print(f"\n{'='*60}")
    print(f"Starting Hyperparameter Search...")
    print(f"{'='*60}")
    
    start_time = datetime.datetime.now()
    
    for trial in range(1, num_trials + 1):
        # Random search
        lr = random.choice(learning_rates)
        batch_size = random.choice(batch_sizes)
        dropout = random.choice(dropout_rates)
        
        loss, accuracy, training_time = train_with_hyperparameters(trial, lr, batch_size, dropout)
        
        results.append({
            'trial': trial,
            'learning_rate': lr,
            'batch_size': batch_size,
            'dropout': dropout,
            'final_loss': loss,
            'final_accuracy': accuracy,
            'training_time': training_time
        })
        
        print(f"    Final: Loss={loss:.4f}, Acc={accuracy:.4f}")
    
    end_time = datetime.datetime.now()
    duration = (end_time - start_time).total_seconds()
    
    # Find best configuration
    best_trial = min(results, key=lambda x: x['final_loss'])
    best_accuracy_trial = max(results, key=lambda x: x['final_accuracy'])
    
    print(f"\n{'='*60}")
    print(f"Hyperparameter Tuning Completed!")
    print(f"{'='*60}")
    print(f"Total Duration: {duration:.2f}s ({duration/60:.2f} minutes)")
    print(f"Trials Completed: {num_trials}")
    
    print(f"\nBest Configuration (by loss):")
    print(f"  Learning Rate: {best_trial['learning_rate']:.2e}")
    print(f"  Batch Size: {best_trial['batch_size']}")
    print(f"  Dropout: {best_trial['dropout']:.2f}")
    print(f"  Final Loss: {best_trial['final_loss']:.4f}")
    print(f"  Final Accuracy: {best_trial['final_accuracy']:.4f}")
    
    if best_accuracy_trial['trial'] != best_trial['trial']:
        print(f"\nBest Configuration (by accuracy):")
        print(f"  Learning Rate: {best_accuracy_trial['learning_rate']:.2e}")
        print(f"  Batch Size: {best_accuracy_trial['batch_size']}")
        print(f"  Dropout: {best_accuracy_trial['dropout']:.2f}")
        print(f"  Final Accuracy: {best_accuracy_trial['final_accuracy']:.4f}")
    
    # Save results
    with open('/output/hyperparameter_results.csv', 'w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=['trial', 'learning_rate', 'batch_size', 'dropout', 
                                                'final_loss', 'final_accuracy', 'training_time'])
        writer.writeheader()
        for r in results:
            writer.writerow({
                'trial': r['trial'],
                'learning_rate': f"{r['learning_rate']:.2e}",
                'batch_size': r['batch_size'],
                'dropout': f"{r['dropout']:.2f}",
                'final_loss': f"{r['final_loss']:.4f}",
                'final_accuracy': f"{r['final_accuracy']:.4f}",
                'training_time': f"{r['training_time']:.2f}"
            })
    
    print(f"\n✓ Generated hyperparameter_results.csv")
    print(f"[{datetime.datetime.now()}] Hyperparameter Tuning completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_hyperparameter_tuning())
