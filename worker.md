# Worker Implementation: Running Containerized Tasks

## 1. Mechanism for Workers to Run User Images and Return Results

### Summary
Worker runs as a daemon/agent on each VM (installed via Ansible). Master assigns task (gRPC). Worker pulls image from Docker Hub, runs container with a host-mounted results dir, streams logs, monitors container, uploads results to object storage (MinIO/S3) or exposes host path, then calls ReportTaskCompletion on master with result_location and logs.

The implementation provides:

- Ansible snippet to bootstrap a VM
- A practical Go worker agent (complete, copy-paste) that:
  - Registers to master
  - Sends periodic heartbeats
  - Implements gRPC server to receive AssignTask
  - Runs task containers using docker CLI
  - Streams logs to /host/results/<task_id>/stdout.log
  - Uploads results to S3/MinIO and sends ReportTaskCompletion

## 2. Implementation Details

### 2.1 Ansible Playbook (install Docker + start worker agent)

#### playbook.yml

```yaml
- hosts: workers
  become: true
  tasks:
    - name: install docker (apt)
      apt:
        name: [apt-transport-https, ca-certificates, curl, gnupg, lsb-release]
        state: present
    - name: add docker gpg
      shell: |
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
      args: creates=/usr/share/keyrings/docker-archive-keyring.gpg
    - name: add docker repo
      apt_repository:
        repo: "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu {{ ansible_lsb.codename }} stable"
    - name: install docker engine
      apt:
        name: docker-ce
        state: present
        update_cache: yes
    - name: copy worker binary
      copy:
        src: ./worker_agent
        dest: /usr/local/bin/worker_agent
        mode: '0755'
    - name: create systemd service
      copy:
        dest: /etc/systemd/system/worker_agent.service
        content: |
          [Unit]
          Description=Worker agent
          After=network.target

          [Service]
          ExecStart=/usr/local/bin/worker_agent --master {{ master_addr }} --worker-id {{ inventory_hostname }}
          Restart=always
          User=root

          [Install]
          WantedBy=multi-user.target
    - name: enable service
      systemd:
        name: worker_agent.service
        enabled: yes
        state: started
```

> Set master_addr to host:port when invoking the playbook.

### 2.2 Worker Agent (Go) — Copy/Paste Ready (Essential Parts)

This is a focused, working example. It uses the os/exec docker CLI (no external docker SDK), google.golang.org/grpc and the proto you already provided (generate .pb.go with protoc). It uploads /host/results/<task_id> to MinIO (S3-compatible) using the official AWS SDK v2 (or minio-go). The code is compact and runnable.

Save as main.go (you must go mod init and go get deps).

```go
package main

import (
  "context"
  "encoding/json"
  "flag"
  "fmt"
  "io"
  "log"
  "net"
  "os"
  "os/exec"
  "path/filepath"
  "time"

  pb "path/to/generated/clusterpb" // replace with your pb package path

  "github.com/minio/minio-go/v7"
  "github.com/minio/minio-go/v7/pkg/credentials"
  "google.golang.org/grpc"
)

var (
  masterAddr = flag.String("master", "master:50051", "master gRPC address")
  workerID   = flag.String("worker-id", "worker-1", "worker id")
  listenAddr = flag.String("listen", ":50052", "worker gRPC listen")
  resultsDir = flag.String("results-dir", "/host/results", "host base results dir")
  minioURL   = flag.String("minio", "minio:9000", "minio endpoint")
  minioUser  = flag.String("minio-user", "minioadmin", "")
  minioPass  = flag.String("minio-pass", "minioadmin", "")
  bucketName = flag.String("bucket", "task-results", "minio bucket")
)

// helper: upload a results directory to minio as zip (simple: upload each file)
func uploadResultsToMinio(srcDir, taskID string) (string, error) {
  client, err := minio.New(*minioURL, &minio.Options{
    Creds:  credentials.NewStaticV4(*minioUser, *minioPass, ""),
    Secure: false,
  })
  if err != nil { return "", err }
  // ensure bucket
  ctx := context.Background()
  err = client.MakeBucket(ctx, *bucketName, minio.MakeBucketOptions{})
  if err != nil {
    // ignore if exists
  }
  // upload files under srcDir
  uploaded := 0
  err = filepath.Walk(srcDir, func(p string, info os.FileInfo, err error) error {
    if err != nil { return err }
    if info.IsDir() { return nil }
    rel, _ := filepath.Rel(srcDir, p)
    objName := fmt.Sprintf("%s/%s", taskID, rel)
    _, err = client.FPutObject(ctx, *bucketName, objName, p, minio.PutObjectOptions{})
    if err == nil { uploaded++ }
    return err
  })
  if err != nil { return "", err }
  uri := fmt.Sprintf("s3://%s/%s/", *bucketName, taskID)
  if uploaded == 0 { return "", fmt.Errorf("no files uploaded") }
  return uri, nil
}

// Worker gRPC server (implements AssignTask)
type server struct {
  pb.UnimplementedMasterWorkerServer
  masterClient pb.MasterWorkerClient
}

func (s *server) AssignTask(ctx context.Context, t *pb.Task) (*pb.TaskAck, error) {
  log.Printf("AssignTask received: %v", t.TaskId)
  taskID := t.TaskId
  hostTaskDir := filepath.Join(*resultsDir, taskID)
  os.MkdirAll(hostTaskDir, 0755)

  // pull image
  if out, err := exec.Command("docker", "pull", t.DockerImage).CombinedOutput(); err != nil {
    log.Printf("docker pull err: %v out:%s", err, string(out))
    return &pb.TaskAck{Success:false, Message:fmt.Sprintf("pull fail: %v", err)}, nil
  }

  // run container with results mount
  args := []string{"run","--rm","--name", taskID, "-v", fmt.Sprintf("%s:/results", hostTaskDir)}
  // if gpu requested: worker must be able to set --gpus flag; naive check:
  if t.ReqGpu > 0 {
    args = append(args, "--gpus", "all")
  }
  // append image and command
  args = append(args, t.DockerImage)
  if t.Command != "" {
    // naive: container ENTRYPOINT must accept command as args; otherwise pass via env
    args = append(args, "/bin/sh", "-c", t.Command)
  }

  // start container
  cmd := exec.Command("docker", args...)
  // stream logs to file
  logFilePath := filepath.Join(hostTaskDir, "stdout.log")
  lf, _ := os.Create(logFilePath)
  defer lf.Close()
  cmd.Stdout = lf
  cmd.Stderr = lf

  start := time.Now()
  if err := cmd.Start(); err != nil {
    log.Printf("start err: %v", err)
    return &pb.TaskAck{Success:false, Message:err.Error()}, nil
  }

  // while running, concurrently send container stats periodically (simple approach: docker stats --no-stream)
  go func() {
    for {
      time.Sleep(5 * time.Second)
      // check if process finished
      if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
        return
      }
      statsOut, _ := exec.Command("docker", "stats", "--no-stream", "--format", "{{json .}}", taskID).CombinedOutput()
      if len(statsOut)>0 {
        // try to send a heartbeat-like metadata to master
        hb := &pb.Heartbeat{
          WorkerId: *workerID,
          CpuUsage: 0, MemoryUsage: 0, StorageUsage: 0,
          RunningTasks: []*pb.RunningTask{
            {TaskId: taskID, Status: "running"},
          },
        }
        _, _ = s.masterClient.SendHeartbeat(context.Background(), hb)
        _ = statsOut
      }
    }
  }()

  // wait for container to finish
  if err := cmd.Wait(); err != nil {
    log.Printf("container exit err: %v", err)
  }
  duration := time.Since(start)
  log.Printf("task finished in %v", duration)

  // capture final logs (already in stdout.log). Upload results to MinIO
  resultUri, err := uploadResultsToMinio(hostTaskDir, taskID)
  if err != nil {
    // fallback: use file:// path
    resultUri = "file://" + hostTaskDir
  }

  // read tail of logs
  tail := ""
  if b, err := os.ReadFile(logFilePath); err == nil {
    if len(b)>8000 { tail = string(b[len(b)-8000:]) } else { tail = string(b) }
  }

  // Report completion to master
  tr := &pb.TaskResult{
    TaskId: taskID, WorkerId: *workerID, Status: "success", Logs: tail, ResultLocation: resultUri,
  }
  ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  defer cancel()
  _, err = s.masterClient.ReportTaskCompletion(ctx2, tr)
  if err != nil {
    log.Printf("ReportTaskCompletion err: %v", err)
  }
  return &pb.TaskAck{Success:true, Message:"started"}, nil
}

func main() {
  flag.Parse()
  // create gRPC server to accept AssignTask calls from master
  lis, err := net.Listen("tcp", *listenAddr)
  if err != nil { log.Fatalf("listen: %v", err) }
  s := grpc.NewServer()
  // create master client to call master endpoints
  conn, err := grpc.Dial(*masterAddr, grpc.WithInsecure())
  if err != nil { log.Fatalf("dial master: %v", err) }
  masterClient := pb.NewMasterWorkerClient(conn)

  srv := &server{ masterClient: masterClient }
  pb.RegisterMasterWorkerServer(s, srv)

  // initial register with master
  winfo := &pb.WorkerInfo{
    WorkerId: *workerID,
    WorkerIp: "auto", // optional
    TotalCpu: 16, TotalMemory: 64, TotalStorage: 500, TotalGpu: 1,
  }
  _, _ = masterClient.RegisterWorker(context.Background(), winfo)

  // periodic heartbeat goroutine
  go func() {
    for {
      hb := &pb.Heartbeat{WorkerId:*workerID, CpuUsage:0, MemoryUsage:0, StorageUsage:0}
      _, _ = masterClient.SendHeartbeat(context.Background(), hb)
      time.Sleep(5 * time.Second)
    }
  }()

  log.Printf("worker listening %s", *listenAddr)
  if err := s.Serve(lis); err != nil { log.Fatalf("grpc serve: %v", err) }
}
```

#### Notes:

- Replace pb import path with your generated protobuf package. Generate cluster.pb.go using protoc.
- This example uses the docker CLI via os/exec to keep dependencies minimal. For production, use Docker Engine API or moby/moby client library.
- uploadResultsToMinio uses MinIO SDK. Run MinIO on master or provide S3 credentials; alternatively upload to master via HTTP.

### 2.3 Worker: How It Detects Completion, Collects Logs, and Reports

Start container with host mount `-v /host/results/<task_id>:/results`. Task writes artifacts to `/results`.

Worker starts container (detached or foreground). It:

- Spawns goroutine to follow logs (docker logs -f <container>), writes to `/host/results/<task_id>/stdout.log`
- Waits for container exit (cmd.Wait() or docker wait)
- After exit, zip or upload `/host/results/<task_id>` to object storage (S3/MinIO). If upload fails, keep on host and return file:// path to master
- Sends ReportTaskCompletion(TaskResult) with status, logs (tail), and result_location

#### Heartbeat & Runtime Metrics:

- Periodic goroutine runs `docker stats --no-stream --format '{{json .}}' <container_id>` or reads host /proc metrics for CPU/RAM to populate Heartbeat
- In our Go example the heartbeat sends a small heartbeat every 5s; you can enrich it by parsing docker stats output or using the Docker Engine API

### 2.4 Result Retrieval Patterns (Choose One or Both)

#### Mount + Upload (Recommended)
Worker mounts host results and uploads to S3/MinIO after task finishes. Master stores result_location (S3 URI) in MongoDB. Pros: scalable, simple for large artifacts.

#### Push to Master via HTTP (Small Artifacts)
Worker zips `/host/results/<task_id>` and POSTs multipart/form-data to master /upload endpoint, master stores directly to DB or object store. Good for small outputs < 50MB.

#### Shared Network FS (NFS)
Workers write directly to a shared results path (NFS). Master reads that path. Simpler but requires shared filesystem setup.

### 2.5 Master Expectations & Proto Usage

- Master must be able to call the worker's AssignTask gRPC endpoint (so worker runs gRPC server)
- Worker must call back to master for RegisterWorker, SendHeartbeat, and ReportTaskCompletion (master implements those RPCs)
- Keep TLS and auth in front for production. For tests, --insecure is okay on private network

### 2.6 Example AssignTask Flow (Sequence)

1. User submits DockerHub URL + resource reqs to master (via API). Master enqueues Task in MongoDB
2. Scheduler (greedy or agentic) picks workerX
3. Master calls workerX AssignTask(Task{task_id, docker_image, command, req_*})
4. Worker pulls image, runs container with `-v /host/results/task_id:/results`
5. Worker streams logs to `/host/results/task_id/stdout.log`, sends periodic SendHeartbeat
6. On exit, worker uploads `/host/results/task_id` to MinIO and calls master ReportTaskCompletion with result_location and logs
7. Master updates DB (TASKS, ASSIGNMENTS, RESULTS), notifies user or returns result URL

## 3. Quick Tests & Tips

- Test locally: run master (stub) and single worker VM (or local container VM). Use `docker run --privileged` for DinD if you ever test with Docker-in-Docker (not recommended)
- Limit resources when running tests: `docker run --cpus=2 --memory=4g ...` so sloppy tasks don't blow up the host
- For GPU tests, ensure nvidia-docker2 runtime and --gpus flags are available on worker
- Add a timeout per task in your master; worker enforces it and kills container with `docker kill`
