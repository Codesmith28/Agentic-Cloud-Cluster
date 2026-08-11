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

package testworkflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComposeCommandEnvDefaults(t *testing.T) {
	t.Setenv("GF_ADMIN_USER", "")
	t.Setenv("GF_ADMIN_PASSWORD", "")

	env := composeCommandEnv(nil, t.TempDir())
	if got := env["GF_ADMIN_USER"]; got != "admin" {
		t.Fatalf("unexpected GF_ADMIN_USER default: got %q want %q", got, "admin")
	}
	if got := env["GF_ADMIN_PASSWORD"]; got != "password" {
		t.Fatalf("unexpected GF_ADMIN_PASSWORD default: got %q want %q", got, "password")
	}
}

func TestComposeCommandEnvUsesRepoDotEnv(t *testing.T) {
	t.Setenv("GF_ADMIN_USER", "")
	t.Setenv("GF_ADMIN_PASSWORD", "")

	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, ".env"), []byte("GF_ADMIN_USER=local-admin\nGF_ADMIN_PASSWORD=local-secret\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	env := composeCommandEnv(nil, repoRoot)
	if got := env["GF_ADMIN_USER"]; got != "local-admin" {
		t.Fatalf("unexpected GF_ADMIN_USER: got %q want %q", got, "local-admin")
	}
	if got := env["GF_ADMIN_PASSWORD"]; got != "local-secret" {
		t.Fatalf("unexpected GF_ADMIN_PASSWORD: got %q want %q", got, "local-secret")
	}
}

func TestComposeCommandEnvBaseOverridesDotEnv(t *testing.T) {
	t.Setenv("GF_ADMIN_USER", "")
	t.Setenv("GF_ADMIN_PASSWORD", "")

	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, ".env"), []byte("GF_ADMIN_USER=local-admin\nGF_ADMIN_PASSWORD=local-secret\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	base := map[string]string{
		"GF_ADMIN_USER":     "override-admin",
		"GF_ADMIN_PASSWORD": "override-secret",
	}
	env := composeCommandEnv(base, repoRoot)
	if got := env["GF_ADMIN_USER"]; got != "override-admin" {
		t.Fatalf("unexpected GF_ADMIN_USER: got %q want %q", got, "override-admin")
	}
	if got := env["GF_ADMIN_PASSWORD"]; got != "override-secret" {
		t.Fatalf("unexpected GF_ADMIN_PASSWORD: got %q want %q", got, "override-secret")
	}
}
