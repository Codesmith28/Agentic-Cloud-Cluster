package app

import (
	"testing"

	"github.com/Codesmith28/Agentic-Cloud-Cluster/pkg/envutil"
)

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
	if got := envutil.GetEnvBool("WEBUI_ADMIN_RESET_PASSWORD", true); !got {
		t.Fatalf("expected fallback value true")
	}
}
