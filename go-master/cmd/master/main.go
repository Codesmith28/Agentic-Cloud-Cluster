package main

import (
  "context"
  "log"
  "time"

  pb "github.com/Codesmith28/CloudAI/pkg/api" // must match option go_package
  "google.golang.org/grpc"
)

func main() {
  conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
  if err != nil {
    log.Fatal(err)
  }
  defer conn.Close()
  client := pb.NewPlannerClient(conn)

  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()

  // build a trivial PlanRequest
  req := &pb.PlanRequest{}
  req.Tasks = []*pb.Task{{Id:"t1", CpuReq:1.0, MemMb:256, EstimatedSec:10}}
  // call planner
  resp, err := client.Plan(ctx, req)
  if err != nil {
    log.Fatalf("Plan RPC error: %v", err)
  }
  log.Printf("Plan reply: %v", resp.StatusMessage)
}
