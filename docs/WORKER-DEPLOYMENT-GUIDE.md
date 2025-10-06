# Worker Deployment Guide

Quick reference for deploying CloudAI workers in different environments.

---

## 🐳 Docker Compose Deployment (Local/Testing)

### Prerequisites
- Docker 24.0+
- Docker Compose 2.0+
- 8GB+ RAM recommended

### Quick Start

```bash
# 1. Start the entire cluster
./scripts/start-cluster.sh

# 2. Verify cluster is running
docker-compose ps

# 3. Run tests
./scripts/test-cluster.sh

# 4. View logs
docker-compose logs -f master
docker-compose logs -f worker-1

# 5. Stop cluster
docker-compose down
```

### Configuration

Edit `docker-compose.yml` to adjust worker resources:

```yaml
worker-1:
  environment:
    - WORKER_ID=worker-1
    - TOTAL_CPU=8.0      # Change CPU cores
    - TOTAL_MEM=16384    # Change memory (MB)
    - TOTAL_GPU=1        # Change GPU count
```

### Useful Commands

```bash
# Rebuild images after code changes
docker-compose build

# Restart specific service
docker-compose restart worker-1

# View CouchDB data
open http://localhost:5984/_utils

# Execute command in container
docker exec -it cloudai-master /bin/sh
docker exec cloudai-worker-1 docker ps  # See running tasks

# Clean up everything
docker-compose down -v  # Remove volumes too
```

---

## 🖥️ Ansible Deployment (Production/Bare Metal)

### Prerequisites
- Ansible 2.9+
- SSH access to worker nodes
- Ubuntu 20.04+ or similar Linux distribution

### Setup

1. **Update Inventory**

Edit `ansible/inventory/production.yml`:

```yaml
workers:
  hosts:
    worker-1:
      ansible_host: 192.168.1.101  # Your server IP
      worker_cpu: 8.0
      worker_mem: 16384
      worker_gpu: 1
      worker_port: 60001
```

2. **Configure SSH Access**

```bash
# Test connectivity
ansible workers -i ansible/inventory/production.yml -m ping

# If SSH key needed
ssh-copy-id ubuntu@192.168.1.101
```

3. **Deploy Workers**

```bash
# Install and configure workers
ansible-playbook -i ansible/inventory/production.yml \
  ansible/playbooks/setup-workers.yml

# Check status
ansible workers -i ansible/inventory/production.yml \
  -m shell -a 'systemctl status cloudai-worker'
```

### Useful Commands

```bash
# View worker logs
ansible workers -i ansible/inventory/production.yml \
  -m shell -a 'journalctl -u cloudai-worker -n 50'

# Restart workers
ansible workers -i ansible/inventory/production.yml \
  -m systemd -a 'name=cloudai-worker state=restarted' --become

# Update worker binary
ansible-playbook -i ansible/inventory/production.yml \
  ansible/playbooks/update-workers.yml

# Check Docker is running
ansible workers -i ansible/inventory/production.yml \
  -m shell -a 'docker ps'

# Check worker connectivity to master
ansible workers -i ansible/inventory/production.yml \
  -m shell -a 'nc -zv {{ master_host }} {{ master_port }}'
```

---

## 🔄 Worker Communication Flow

```
1. Worker Startup
   ↓
2. Connect to Master (gRPC)
   ↓
3. Register (send ID, resources)
   ↓
4. Start gRPC Server (listen for assignments)
   ↓
5. Send Heartbeats (every 10s)
   ↓
6. Receive Task Assignment
   ↓
7. Execute Task (Docker container)
   ↓
8. Report Completion
```

---

## 🐛 Troubleshooting

### Docker Deployment Issues

**Problem:** Worker can't connect to master
```bash
# Check network
docker-compose exec worker-1 ping master
docker-compose exec worker-1 nc -zv master 50051

# View master logs
docker-compose logs master | grep "Registering worker"
```

**Problem:** Docker-in-Docker not working
```bash
# Verify Docker socket is mounted
docker-compose exec worker-1 docker ps

# Check if dockerd is running
docker-compose exec worker-1 ps aux | grep dockerd
```

**Problem:** Task execution fails
```bash
# Check worker logs
docker-compose logs worker-1 | grep "Executing task"

# See what containers are running
docker-compose exec worker-1 docker ps -a
```

### Ansible Deployment Issues

**Problem:** SSH connection fails
```bash
# Test SSH manually
ssh -i ~/.ssh/cloudai-workers.pem ubuntu@192.168.1.101

# Verify ansible_host is correct
ansible-inventory -i ansible/inventory/production.yml --list
```

**Problem:** Worker service won't start
```bash
# Check service status
ansible workers -m shell -a 'systemctl status cloudai-worker' -i ansible/inventory/production.yml

# View full logs
ansible workers -m shell -a 'journalctl -u cloudai-worker -n 100' -i ansible/inventory/production.yml

# Check if binary exists
ansible workers -m shell -a 'ls -la /usr/local/bin/cloudai-worker' -i ansible/inventory/production.yml
```

**Problem:** Docker not installed
```bash
# Manually install Docker on worker
ansible workers -m shell -a 'docker --version' -i ansible/inventory/production.yml

# Re-run Docker installation tasks
ansible-playbook -i ansible/inventory/production.yml \
  ansible/playbooks/setup-workers.yml --tags docker
```

---

## 📊 Monitoring

### Docker Deployment

```bash
# Watch worker resource usage
docker stats cloudai-worker-1 cloudai-worker-2 cloudai-worker-3

# Monitor task execution
watch -n 1 'docker-compose exec worker-1 docker ps'

# View metrics
curl http://localhost:8080/metrics  # If Prometheus enabled
```

### Ansible Deployment

```bash
# Monitor system resources
ansible workers -i ansible/inventory/production.yml \
  -m shell -a 'top -bn1 | head -20'

# Check worker process
ansible workers -i ansible/inventory/production.yml \
  -m shell -a 'ps aux | grep cloudai-worker'

# View network connections
ansible workers -i ansible/inventory/production.yml \
  -m shell -a 'netstat -tulpn | grep 60001'
```

---

## 🔐 Security Considerations

### Docker Deployment
- Workers run in privileged mode (required for DinD)
- Docker socket is exposed to containers
- **Use only in trusted/development environments**

### Production Deployment
- Use SSH key authentication (not passwords)
- Configure firewall rules (only allow necessary ports)
- Use TLS for gRPC communication (recommended)
- Regularly update worker binaries
- Monitor for suspicious task execution

---

## 📈 Scaling

### Add Workers (Docker)

Edit `docker-compose.yml`:

```yaml
worker-4:
  build:
    context: .
    dockerfile: Dockerfile.worker
  container_name: cloudai-worker-4
  environment:
    - WORKER_ID=worker-4
    - WORKER_PORT=60004
    - MASTER_ADDRESS=master:50051
    - TOTAL_CPU=8.0
    - TOTAL_MEM=8192
  # ... rest of config
```

Then: `docker-compose up -d worker-4`

### Add Workers (Ansible)

Edit `ansible/inventory/production.yml`:

```yaml
worker-4:
  ansible_host: 192.168.1.104
  worker_id: worker-4
  worker_cpu: 16.0
  worker_mem: 32768
```

Then: `ansible-playbook -i ansible/inventory/production.yml ansible/playbooks/setup-workers.yml --limit worker-4`

---

## 🧪 Testing Checklist

- [ ] Workers register with master on startup
- [ ] Workers appear in `ListWorkers` API response
- [ ] Master can assign tasks to workers
- [ ] Tasks execute successfully in Docker containers
- [ ] Workers report task completion
- [ ] Heartbeats received every 10s
- [ ] Worker survives master restart (reconnects automatically)
- [ ] Resources are properly tracked (CPU, memory, GPU)
- [ ] Logs are accessible and meaningful

---

## 📚 Additional Resources

- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [Ansible Documentation](https://docs.ansible.com/)
- [Docker-in-Docker Best Practices](https://jpetazzo.github.io/2015/09/03/do-not-use-docker-in-docker-for-ci/)
- [Systemd Service Management](https://www.freedesktop.org/software/systemd/man/systemd.service.html)

---

**Need Help?** 
- Check Sprint.md Task 2.6 for implementation details
- See `docs/Sprint-2-Deployment-Addition.md` for architecture overview
- Review worker logs for debugging information
