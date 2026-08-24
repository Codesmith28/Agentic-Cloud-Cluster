package app

import (
	"testing"

	"master/internal/config"
	"master/internal/system"
)

func TestRunTestCommandUsageAndList(t *testing.T) {
	cfg := &config.Config{}

	// Empty args should return usage exit code 2
	if code := RunTestCommand(cfg, []string{}); code != 2 {
		t.Fatalf("expected exit code 2 for empty args, got %d", code)
	}

	// 'list' subcommand should return exit code 0
	if code := RunTestCommand(cfg, []string{"list"}); code != 0 {
		t.Fatalf("expected exit code 0 for 'list', got %d", code)
	}

	// Invalid profile should return exit code 2
	if code := RunTestCommand(cfg, []string{"run", "invalid-profile"}); code != 2 {
		t.Fatalf("expected exit code 2 for invalid profile, got %d", code)
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
