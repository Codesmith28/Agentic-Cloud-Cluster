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
	"testing"

	"master/internal/config"
	"master/internal/system"
)

func TestParseNonInteractiveTestRunOptionsAcceptsSchedulerVariants(t *testing.T) {
	opts, err := parseNonInteractiveTestRunOptions([]string{"-scheduler", "RTS", "-keep-env", "-ui-smoke"})
	if err != nil {
		t.Fatalf("parseNonInteractiveTestRunOptions returned error: %v", err)
	}
	if opts.Scheduler != "rts" {
		t.Fatalf("expected scheduler rts, got %q", opts.Scheduler)
	}
	if !opts.KeepEnvironment {
		t.Fatalf("expected KeepEnvironment=true")
	}
	if !opts.EnableUISmoke {
		t.Fatalf("expected EnableUISmoke=true")
	}
}

func TestParseNonInteractiveTestRunOptionsRejectsInvalidScheduler(t *testing.T) {
	if _, err := parseNonInteractiveTestRunOptions([]string{"-scheduler", "PPO"}); err == nil {
		t.Fatal("expected error for unsupported scheduler")
	}
}

func TestResolveMasterAddressesDefaults(t *testing.T) {
	sysInfo := &system.SystemInfo{IPAddresses: []string{"10.0.0.8"}}
	cfg := &config.Config{GRPCPort: ":50051"}

	bind, advertise := resolveMasterAddresses(sysInfo, cfg)
	if bind != "10.0.0.8:50051" {
		t.Fatalf("unexpected bind address: %q", bind)
	}
	if advertise != "10.0.0.8:50051" {
		t.Fatalf("unexpected advertise address: %q", advertise)
	}
}

func TestResolveMasterAddressesOverrides(t *testing.T) {
	sysInfo := &system.SystemInfo{IPAddresses: []string{"10.0.0.8"}}
	cfg := &config.Config{
		GRPCPort:       ":50051",
		MasterBindAddr: " 0.0.0.0:50052 ",
		MasterAdvAddr:  " master.example:50052 ",
	}

	bind, advertise := resolveMasterAddresses(sysInfo, cfg)
	if bind != "0.0.0.0:50052" {
		t.Fatalf("unexpected bind address: %q", bind)
	}
	if advertise != "master.example:50052" {
		t.Fatalf("unexpected advertise address: %q", advertise)
	}
}

func TestRunNonInteractiveTestCommandRejectsUnknownSuite(t *testing.T) {
	cfg := &config.Config{GRPCPort: ":50051", HTTPPort: ":8080", SchedulerAlgo: "RTS"}
	code := runNonInteractiveTestCommand(cfg, []string{"run", "unknown-suite"})
	if code != 2 {
		t.Fatalf("expected exit code 2 for unsupported suite, got %d", code)
	}
}
