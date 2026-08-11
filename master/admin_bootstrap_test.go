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

import "testing"

func TestLoadWebUIAdminBootstrapConfigDefaults(t *testing.T) {
	t.Setenv("WEBUI_ADMIN_NAME", "")
	t.Setenv("WEBUI_ADMIN_EMAIL", "")
	t.Setenv("WEBUI_ADMIN_PASSWORD", "")
	t.Setenv("WEBUI_ADMIN_RESET_PASSWORD", "")

	cfg := loadWebUIAdminBootstrapConfig()
	if cfg.Name != defaultWebUIAdminName {
		t.Fatalf("expected default name %q, got %q", defaultWebUIAdminName, cfg.Name)
	}
	if cfg.Email != defaultWebUIAdminEmail {
		t.Fatalf("expected default email %q, got %q", defaultWebUIAdminEmail, cfg.Email)
	}
	if cfg.Password != defaultWebUIAdminPassword {
		t.Fatalf("expected default password fallback")
	}
	if cfg.ResetPassword {
		t.Fatalf("expected reset password default false")
	}
}

func TestLoadWebUIAdminBootstrapConfigOverrides(t *testing.T) {
	t.Setenv("WEBUI_ADMIN_NAME", "Cluster Admin")
	t.Setenv("WEBUI_ADMIN_EMAIL", "admin@example.com")
	t.Setenv("WEBUI_ADMIN_PASSWORD", "SuperSecurePassword123!")
	t.Setenv("WEBUI_ADMIN_RESET_PASSWORD", "true")

	cfg := loadWebUIAdminBootstrapConfig()
	if cfg.Name != "Cluster Admin" {
		t.Fatalf("unexpected name: %q", cfg.Name)
	}
	if cfg.Email != "admin@example.com" {
		t.Fatalf("unexpected email: %q", cfg.Email)
	}
	if cfg.Password != "SuperSecurePassword123!" {
		t.Fatalf("unexpected password")
	}
	if !cfg.ResetPassword {
		t.Fatalf("expected reset password true")
	}
}

func TestGetBoolEnvWithDefaultInvalidValue(t *testing.T) {
	t.Setenv("WEBUI_ADMIN_RESET_PASSWORD", "not-a-bool")
	if got := getBoolEnvWithDefault("WEBUI_ADMIN_RESET_PASSWORD", true); !got {
		t.Fatalf("expected fallback value true")
	}
}
