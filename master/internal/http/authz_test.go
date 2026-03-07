package http

import "testing"

func TestCanReadTaskResult(t *testing.T) {
	user := &AuthPrincipal{Email: "alice@example.com", Role: "user"}
	if !canReadTaskResult(user, "alice@example.com") {
		t.Fatal("owner should be able to read task results")
	}
	if canReadTaskResult(user, "bob@example.com") {
		t.Fatal("non-owner should not be able to read task results")
	}
}

func TestCanOperateTask(t *testing.T) {
	admin := &AuthPrincipal{Email: "admin@example.com", Role: "admin"}
	user := &AuthPrincipal{Email: "alice@example.com", Role: "user"}

	if !canOperateTask(user, "alice@example.com") {
		t.Fatal("owner should be able to operate task")
	}
	if canOperateTask(user, "bob@example.com") {
		t.Fatal("user should not operate other user's task")
	}
	if !canOperateTask(admin, "bob@example.com") {
		t.Fatal("admin should operate any task")
	}
}

func TestRequireBreakglass(t *testing.T) {
	admin := &AuthPrincipal{Email: "admin@example.com", Role: "admin"}
	user := &AuthPrincipal{Email: "alice@example.com", Role: "user"}

	if err := requireBreakglass(user, "alice@example.com", ""); err != nil {
		t.Fatalf("owner should not need break-glass: %v", err)
	}
	if err := requireBreakglass(user, "bob@example.com", "debug"); err == nil {
		t.Fatal("non-admin non-owner should be denied")
	}
	if err := requireBreakglass(admin, "bob@example.com", ""); err == nil {
		t.Fatal("admin should require break-glass reason for cross-user result read")
	}
	if err := requireBreakglass(admin, "bob@example.com", "incident-response"); err != nil {
		t.Fatalf("admin with reason should pass break-glass: %v", err)
	}
}
