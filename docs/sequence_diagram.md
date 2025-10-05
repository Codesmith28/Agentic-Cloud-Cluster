Sequence Diagram - AI-Driven Agentic Scheduler

Actors:
- Client
- Master
- Planner (Python)
- Worker(s)

---

Step 1: Task Submission
Client -> Master: SubmitTask(task)
Master -> TaskQueue: Enqueue(task)
Master -> WorkerRegistry: GetSnapshotOfWorkers()
Master -> Planner: RequestPlan(tasks, workers)

Step 2: Planning
Planner -> AStar / OR-Tools / Predictor: Compute optimal assignment
Planner -> Master: Return PlanResponse(task-worker assignments)

Step 3: Task Dispatch
Master -> Worker1: AssignTask(task1)
Master -> Worker2: AssignTask(task2)
...
Worker1 -> ContainerManager/VMManager: LaunchTask(task1)
Worker2 -> ContainerManager/VMManager: LaunchTask(task2)
...

Step 4: Task Execution and Monitoring
Worker1 -> Master: Heartbeat(status, resource usage)
Worker2 -> Master: Heartbeat(status, resource usage)
...
Worker1 -> Master: ReportTaskCompletion(task1, status)
Worker2 -> Master: ReportTaskCompletion(task2, status)
...

Step 5: Failure Handling & Replanning
Master -> Monitor: CheckWorkerHealth()
Monitor -> Master: DetectFailure(workerX)
Master -> Planner: Replan(failedTasks, availableWorkers)
Planner -> Master: Return UpdatedPlan
Master -> Worker(s): ReassignTasks(failedTasks)

Step 6: Continuous Operation
Loop:
  Client -> Master: SubmitTask(newTask)
  Master -> Planner: RequestPlan(updatedTasks, updatedWorkers)
  Planner -> Master: Return PlanResponse
  Master -> Worker(s): AssignTask(newTask)
  Worker(s) -> Master: ReportTaskCompletion(newTask)
EndLoop
