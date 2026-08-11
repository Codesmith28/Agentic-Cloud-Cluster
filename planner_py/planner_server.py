# Copyright 2025-2026 Sarthak Siddhpura
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# planner_py/planner_server.py
import grpc
from concurrent import futures
import time

import scheduler_pb2
import scheduler_pb2_grpc

class PlannerServicer(scheduler_pb2_grpc.PlannerServicer):
    def Plan(self, request, context):
        resp = scheduler_pb2.PlanResponse()
        resp.plan_id = "plan-stub-1"
        resp.status_message = "stub plan"
        # trivial round-robin assignment
        for i, t in enumerate(request.tasks):
            a = resp.assignments.add()
            a.task_id = t.id
            if len(request.workers) > 0:
                a.worker_id = request.workers[i % len(request.workers)].id
                a.start_unix = 0
                a.est_duration_sec = int(t.estimated_sec)
        return resp

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    scheduler_pb2_grpc.add_PlannerServicer_to_server(PlannerServicer(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    try:
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        server.stop(0)

if __name__ == "__main__":
    serve()
