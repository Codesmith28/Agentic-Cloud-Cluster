#!/usr/bin/env python3
"""
Workload Type: gpu-training
Description: Simulated reinforcement learning training loop
Resource Requirements: High CPU, high memory, high GPU
Output: None (logs only)
"""

import datetime
import random
import time

def simulate_episode(episode_num, max_steps=200):
    """Simulate one RL episode"""
    total_reward = 0
    steps = 0
    
    for step in range(max_steps):
        time.sleep(0.01)
        
        # Simulate reward (increases over episodes as agent learns)
        base_reward = random.uniform(-1, 10) + (episode_num * 0.1)
        reward = base_reward
        total_reward += reward
        steps += 1
        
        # Simulate episode termination
        if random.random() < 0.05 or reward < -5:
            break
    
    return total_reward, steps

def run_rl_training():
    print(f"[{datetime.datetime.now()}] Starting Reinforcement Learning Training (GPU Training)...")
    print("🖥️  GPU Training Mode - Long Running Task")
    
    num_episodes = 200
    
    print(f"\n✓ Environment: CartPole-v1 (simulated)")
    print(f"✓ Algorithm: PPO (Proximal Policy Optimization)")
    print(f"✓ Episodes: {num_episodes}")
    print(f"✓ Max steps per episode: 200")
    
    episode_rewards = []
    episode_lengths = []
    
    print(f"\n{'='*60}")
    print(f"Starting RL Training...")
    print(f"{'='*60}\n")
    
    start_time = datetime.datetime.now()
    
    for episode in range(1, num_episodes + 1):
        total_reward, steps = simulate_episode(episode)
        episode_rewards.append(total_reward)
        episode_lengths.append(steps)
        
        if episode % 20 == 0:
            avg_reward = sum(episode_rewards[-20:]) / 20
            avg_length = sum(episode_lengths[-20:]) / 20
            print(f"  Episode {episode}/{num_episodes}:")
            print(f"    Avg Reward (last 20): {avg_reward:.2f}")
            print(f"    Avg Length (last 20): {avg_length:.1f}")
            print(f"    Current Episode Reward: {total_reward:.2f}")
    
    end_time = datetime.datetime.now()
    duration = (end_time - start_time).total_seconds()
    
    # Calculate statistics
    total_steps = sum(episode_lengths)
    avg_reward = sum(episode_rewards) / len(episode_rewards)
    max_reward = max(episode_rewards)
    
    # Calculate moving average of last 100 episodes
    final_avg_reward = sum(episode_rewards[-100:]) / min(100, len(episode_rewards))
    
    print(f"\n{'='*60}")
    print(f"RL Training Completed!")
    print(f"{'='*60}")
    print(f"Total Duration: {duration:.2f}s ({duration/60:.2f} minutes)")
    print(f"Total Steps: {total_steps:,}")
    print(f"Average Reward: {avg_reward:.2f}")
    print(f"Max Reward: {max_reward:.2f}")
    print(f"Final 100-episode Average: {final_avg_reward:.2f}")
    
    # Show learning progress
    print(f"\nLearning Progress:")
    print(f"  Episodes 1-50 avg: {sum(episode_rewards[:50])/50:.2f}")
    print(f"  Episodes 51-100 avg: {sum(episode_rewards[50:100])/50:.2f}")
    print(f"  Episodes 101-150 avg: {sum(episode_rewards[100:150])/50:.2f}")
    print(f"  Episodes 151-200 avg: {sum(episode_rewards[150:200])/50:.2f}")
    
    print(f"\n[{datetime.datetime.now()}] RL Training completed ✓")
    return 0

if __name__ == "__main__":
    exit(run_rl_training())
