# Architecture - AI-Driven Agentic Scheduler

This document describes the **High-Level Design (HLD)** of the AI-driven agentic scheduler project.

---

## System Overview

The system is composed of three main parts:

1. **Master Node (Go)**
   - Acts as the brain of the system for coordination and control.
   - Accepts client task submissions.
   - Maintains a **task queue** where incoming jobs are stored.
   - Tracks all available workers in the **worker registry**.
   - Calls the **planner service** (Python) to compute optimized scheduling decisions.
   - Dispatches tasks to workers based on planner output.
   - Monitors worker health via heartbeats and handles worker failures.

2. **Planner Service (Python)**
   - Runs as a separate service, connected via gRPC.
   - Implements **agentic scheduling algorithms**:
     - **A\*** search for forward state-space planning.
     - **Constraint-based scheduling** using OR-Tools CP-SAT solver.
     - **Replanner** for repairing schedules when workers fail or new tasks arrive.
     - **Predictor** for estimating runtime using ML models.
   - Receives the current state (tasks + workers) from the Master.
   - Returns an optimized **task-to-worker assignment plan**.

3. **Worker Nodes (Go)**
   - Execute assigned tasks.
   - Tasks can be launched inside **containers (Docker)** or **VMs (KVM/Xen)** depending on requirements.
   - Report **heartbeat messages** with current resource availability.
   - Send **completion or failure messages** after executing tasks.

---

## System Flow

1. **Task Submission**
   - A client submits tasks to the Master.
   - The Master stores tasks in its internal queue.

2. **Planning**
   - The Master collects the current state of all workers from the registry.
   - It sends this state along with pending tasks to the Planner service.
   - The Planner computes an optimal schedule (mapping tasks to workers) using AI planning algorithms.
   - The schedule is returned to the Master.

3. **Execution**
   - The Master sends assignments to the appropriate Worker nodes.
   - Workers launch tasks inside containers or VMs.
   - Workers send acknowledgements and progress updates.

4. **Monitoring & Fault Handling**
   - Workers send periodic heartbeats to the Master.
   - If a worker fails or becomes unresponsive, the Master marks its tasks as incomplete.
   - The Master requests a new plan from the Planner to reassign those tasks (replanning).

---

## Design Highlights

- **Hybrid Implementation**
  - **Go** for high-performance system orchestration and worker management.
  - **Python** for AI-based planning with access to mature libraries.

- **gRPC for Communication**
  - Strongly typed, language-agnostic communication between Master and Planner.
  - Enables seamless scaling of the Planner service independently.

- **Agentic Planning**
  - Uses **goal-based agents** that reason about the future state of the system.
  - Unlike Kubernetes’ rule-based scheduler, this system plans with **explicit goals and heuristics**.

- **Resilience**
  - Supports **incremental replanning** when resources change or tasks fail.
  - Provides higher adaptability in dynamic and heterogeneous environments.

---

## Benefits Over Traditional Schedulers

- **Kubernetes Scheduler**:  
  - Rule-based, greedy scheduling with limited lookahead.  
  - Cannot easily optimize across multiple constraints (deadlines, utilization, energy).  

- **Agentic Scheduler (This Project)**:  
  - Plans ahead using AI techniques.  
  - Optimizes objectives such as makespan, utilization, and SLA deadlines.  
  - Repairs plans dynamically on worker failures.  
  - Integrates prediction models to improve decision quality over time.  

---
