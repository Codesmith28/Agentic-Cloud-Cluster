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
