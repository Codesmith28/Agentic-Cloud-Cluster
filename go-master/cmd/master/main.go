// Copyright 2025-2026 Sarthak Siddhpura
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
