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
	"encoding/json"
	"fmt"
	"net/http"
)

// AuthMiddleware wraps a handler and requires authentication
func (h *AuthHandler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, err := h.AuthenticateRequest(r)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": fmt.Sprintf("Unauthorized: %v", err),
			})
			return
		}

		ctx := withAuthPrincipal(r.Context(), principal)

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// AuthenticateRequest validates auth cookie/JWT and builds the request principal.
func (h *AuthHandler) AuthenticateRequest(r *http.Request) (*AuthPrincipal, error) {
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return nil, fmt.Errorf("no authentication token")
	}

	claims, err := h.VerifyToken(cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	principal := &AuthPrincipal{
		Email: claims.Email,
		Name:  claims.Name,
		Role:  normalizeRole(claims.Role),
	}

	// Keep role authoritative by reading from DB when available.
	if h.userDB != nil && principal.Email != "" {
		user, err := h.userDB.GetUserByEmail(principal.Email)
		if err == nil && user != nil {
			principal.Role = normalizeRole(user.Role)
		}
	}

	return principal, nil
}
