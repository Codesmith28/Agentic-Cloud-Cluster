#!/bin/bash

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

# Create directories
mkdir -p proto
mkdir -p go-master/cmd/master
mkdir -p go-master/cmd/worker
mkdir -p go-master/pkg/api
mkdir -p go-master/pkg/scheduler
mkdir -p go-master/pkg/workerregistry
mkdir -p go-master/pkg/taskqueue
mkdir -p go-master/pkg/execution
mkdir -p go-master/pkg/persistence
mkdir -p go-master/pkg/monitor
mkdir -p go-master/pkg/vm_manager
mkdir -p go-master/pkg/container_manager
mkdir -p go-master/pkg/testbench
mkdir -p planner_py/planner
mkdir -p docs
mkdir -p ci

# Create empty files
touch proto/scheduler.proto
touch go-master/cmd/master/main.go
touch go-master/cmd/worker/main.go
touch go-master/pkg/api/api.go
touch go-master/pkg/scheduler/scheduler.go
touch go-master/pkg/workerregistry/registry.go
touch go-master/pkg/taskqueue/queue.go
touch go-master/pkg/execution/executor.go
touch go-master/pkg/persistence/persistence.go
touch go-master/pkg/monitor/monitor.go
touch go-master/pkg/vm_manager/vm.go
touch go-master/pkg/container_manager/container.go
touch go-master/pkg/testbench/testbench.go
touch planner_py/planner_server.py
touch planner_py/planner/__init__.py
touch planner_py/planner/a_star.py
touch planner_py/planner/or_tools_scheduler.py
touch planner_py/planner/replanner.py
touch planner_py/planner/predictor.py
touch planner_py/requirements.txt
touch docs/README.md
touch ci/ci.yml
touch ci/Makefile

echo "Project structure created successfully ✅"
