package http

import (
	"context"
	"encoding/json"
	"net/http"
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

		// Add user info to context
		ctx := context.WithValue(r.Context(), "user_email", claims.Email)
		ctx = context.WithValue(ctx, "user_name", claims.Name)

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
