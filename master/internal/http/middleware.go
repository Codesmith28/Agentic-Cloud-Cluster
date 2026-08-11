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
	"encoding/json"
	"net/http"
)

type contextKey string

const (
	ctxKeyUserEmail contextKey = "user_email"
	ctxKeyUserName  contextKey = "user_name"
)

// AuthMiddleware wraps a handler and requires authentication
func (h *AuthHandler) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get token from cookie
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Unauthorized: No authentication token",
			})
			return
		}

		// Verify token
		claims, err := h.VerifyToken(cookie.Value)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Unauthorized: Invalid token",
			})
			return
		}

		// Add user info to context using typed keys to prevent collisions
		ctx := context.WithValue(r.Context(), ctxKeyUserEmail, claims.Email)
		ctx = context.WithValue(ctx, ctxKeyUserName, claims.Name)

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
