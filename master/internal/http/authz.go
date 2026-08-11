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

package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

type role string

const (
	roleUser  role = "user"
	roleAdmin role = "admin"
)

const (
	breakglassReasonHeader = "X-Breakglass-Reason"
)

type contextKey string

const authPrincipalContextKey contextKey = "auth_principal"

// AuthPrincipal captures authenticated identity available to handlers.
type AuthPrincipal struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func normalizeRole(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(roleAdmin):
		return string(roleAdmin)
	default:
		return string(roleUser)
	}
}

func (p *AuthPrincipal) IsAdmin() bool {
	if p == nil {
		return false
	}
	return normalizeRole(p.Role) == string(roleAdmin)
}

func withAuthPrincipal(ctx context.Context, principal *AuthPrincipal) context.Context {
	return context.WithValue(ctx, authPrincipalContextKey, principal)
}

func getAuthPrincipal(ctx context.Context) (*AuthPrincipal, bool) {
	principal, ok := ctx.Value(authPrincipalContextKey).(*AuthPrincipal)
	if !ok || principal == nil || principal.Email == "" {
		return nil, false
	}
	return principal, true
}

func canReadTaskResult(principal *AuthPrincipal, taskOwner string) bool {
	if principal == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(principal.Email), strings.TrimSpace(taskOwner))
}

func canOperateTask(principal *AuthPrincipal, taskOwner string) bool {
	if principal == nil {
		return false
	}
	if canReadTaskResult(principal, taskOwner) {
		return true
	}
	return principal.IsAdmin()
}

func requireBreakglass(principal *AuthPrincipal, taskOwner, reason string) error {
	if principal == nil {
		return errors.New("missing principal")
	}
	if canReadTaskResult(principal, taskOwner) {
		return nil
	}
	if !principal.IsAdmin() {
		return errors.New("access denied")
	}
	if strings.TrimSpace(reason) == "" {
		return errors.New("break-glass reason required")
	}
	return nil
}

func authorizeTaskResultRead(principal *AuthPrincipal, taskOwner, resource, reason string) error {
	err := requireBreakglass(principal, taskOwner, reason)
	success := err == nil
	if principal != nil && principal.IsAdmin() && !canReadTaskResult(principal, taskOwner) {
		auditBreakglass(principal, taskOwner, resource, reason, success)
	}
	if err != nil {
		if strings.Contains(err.Error(), "break-glass") {
			return fmt.Errorf("forbidden: admin break-glass requires %s header", breakglassReasonHeader)
		}
		return fmt.Errorf("forbidden: result access denied")
	}
	return nil
}

func auditBreakglass(principal *AuthPrincipal, taskOwner, resource, reason string, success bool) {
	if principal == nil {
		return
	}
	status := "DENIED"
	if success {
		status = "ALLOWED"
	}

	log.Printf("[AUDIT_BREAKGLASS] status=%s actor=%s role=%s owner=%s resource=%q reason=%q",
		status, principal.Email, normalizeRole(principal.Role), taskOwner, resource, strings.TrimSpace(reason))
}
