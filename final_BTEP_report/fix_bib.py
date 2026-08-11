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

import re

with open('General/References.bib', 'r') as f:
    content = f.read()

# Fix Q-learning
content = content.replace('title={Q-learning}', 'title={{Q}-learning}')

# Fix Atari
content = content.replace('title={Playing Atari with Deep Reinforcement Learning}', 'title={Playing {Atari} with Deep Reinforcement Learning}')

# Fix Ray
content = content.replace('title = {Ray: A Distributed Framework for Emerging {AI} Applications}', 'title = {{Ray}: A Distributed Framework for Emerging {AI} Applications}')

# Fix Alibaba
content = content.replace('title={Alibaba Cluster Data}', 'title={{Alibaba} Cluster Data}')

# Fix gRPC
content = content.replace('title={Introduction to gRPC}', 'title={Introduction to {gRPC}}')

# Fix Go
content = content.replace('title={Documentation - The Go Programming Language}', 'title={Documentation - The {Go} Programming Language}')

# Fix Docker
content = content.replace('title={Docker Documentation}', 'title={{Docker} Documentation}')

# Fix MongoDB
content = content.replace('title={Introduction to MongoDB}', 'title={Introduction to {MongoDB}}')

# Fix Google and Borg
content = content.replace('title={Large-scale cluster management at Google with Borg}', 'title={Large-scale cluster management at {Google} with {Borg}}')

# Fix Kubernetes
content = content.replace('title = {What is Kubernetes?}', 'title = {What is {Kubernetes}?}')

# Fix Volcano
content = content.replace('title={Volcano: A Cloud Native Batch System}', 'title={{Volcano}: A Cloud Native Batch System}')

# Fix Apache YuniKorn
content = content.replace('title={Apache YuniKorn: A Cloud Native Scheduler}', 'title={{Apache YuniKorn}: A Cloud Native Scheduler}')

# Fix Dask
content = content.replace('title = {Dask: Parallel Computation with Blocked algorithms and Task Scheduling}', 'title = {{Dask}: Parallel Computation with Blocked algorithms and Task Scheduling}')

# Fix Soft Actor-Critic
content = content.replace('title = {Soft Actor-Critic: Off-Policy Maximum Entropy Deep Reinforcement Learning with a Stochastic Actor}', 'title = {{Soft Actor-Critic}: Off-Policy Maximum Entropy Deep Reinforcement Learning with a Stochastic Actor}')

with open('General/References.bib', 'w') as f:
    f.write(content)

