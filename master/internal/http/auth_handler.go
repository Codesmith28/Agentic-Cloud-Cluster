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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"master/internal/db"
)

// maxAuthBodySize limits the request body size for auth endpoints (1MB)
const maxAuthBodySize = 1 << 20

// emailRegex validates basic email format
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	userDB    *db.UserDB
	jwtSecret []byte
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(userDB *db.UserDB) *AuthHandler {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Generate a random 32-byte secret if none is configured
		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			log.Fatalf("FATAL: failed to generate JWT secret: %v", err)
		}
		secret = hex.EncodeToString(randomBytes)
		log.Println("WARNING: JWT_SECRET not set, generated a random secret. Sessions will not survive restarts.")
	}
	if len(secret) < 32 {
		log.Println("WARNING: JWT_SECRET is shorter than 32 characters; consider using a stronger secret.")
	}

	return &AuthHandler{
		userDB:    userDB,
		jwtSecret: []byte(secret),
	}
}

// Claims represents JWT claims
type Claims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest represents login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	Success    bool      `json:"success"`
	Message    string    `json:"message"`
	User       *UserInfo `json:"user,omitempty"`
	VisitCount int       `json:"visit_count,omitempty"`
}

// UserInfo represents safe user information (no password)
type UserInfo struct {
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	VisitCount int       `json:"visit_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// HandleRegister handles user registration
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate input
	if req.Name == "" || req.Email == "" || req.Password == "" {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Name, email, and password are required",
		})
		return
	}

	if !emailRegex.MatchString(req.Email) {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Invalid email format",
		})
		return
	}

	if len(req.Password) < 12 {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Password must be at least 12 characters",
		})
		return
	}

	// Create user
	err := h.userDB.CreateUser(req.Name, req.Email, req.Password)
	if err != nil {
		log.Printf("Failed to create user %s: %v", req.Email, err)
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Failed to create user",
		})
		return
	}

	// Get created user
	user, err := h.userDB.GetUserByEmail(req.Email)
	if err != nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "User created but failed to retrieve",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Registration successful",
		User: &UserInfo{
			Email:      user.Email,
			Name:       user.Name,
			VisitCount: user.VisitCount,
			CreatedAt:  user.CreatedAt,
		},
	})
}

// HandleLogin handles user login
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate credentials
	user, err := h.userDB.ValidateCredentials(req.Email, req.Password)
	if err != nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Invalid email or password",
		})
		return
	}

	// Increment visit count
	if err := h.userDB.IncrementVisitCount(req.Email); err != nil {
		// Log error but don't fail the login
		println("Warning: Failed to increment visit count:", err.Error())
	}

	// Get updated user with new visit count
	user, _ = h.userDB.GetUserByEmail(req.Email)

	// Generate JWT token
	expirationTime := time.Now().Add(24 * time.Hour) // 24 hours
	claims := &Claims{
		Email: user.Email,
		Name:  user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Failed to generate token",
		})
		return
	}

	// Set cookie
	secureCookie := shouldUseSecureAuthCookie(r)
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    tokenString,
		Expires:  expirationTime,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success:    true,
		Message:    "Login successful",
		VisitCount: user.VisitCount,
		User: &UserInfo{
			Email:      user.Email,
			Name:       user.Name,
			VisitCount: user.VisitCount,
			CreatedAt:  user.CreatedAt,
		},
	})
}

// HandleLogout handles user logout
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear cookie
	secureCookie := shouldUseSecureAuthCookie(r)
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "Logout successful",
	})
}

// HandleMe returns current user information
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get email from context (set by middleware)
	email, ok := r.Context().Value(ctxKeyUserEmail).(string)
	if !ok {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	// Get user from database
	user, err := h.userDB.GetUserByEmail(email)
	if err != nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "User not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		Message: "User retrieved successfully",
		User: &UserInfo{
			Email:      user.Email,
			Name:       user.Name,
			VisitCount: user.VisitCount,
			CreatedAt:  user.CreatedAt,
		},
	})
}

// VerifyToken verifies a JWT token and returns the claims
func (h *AuthHandler) VerifyToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method to prevent algorithm confusion attacks
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return h.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}

func shouldUseSecureAuthCookie(r *http.Request) bool {
	if override := strings.TrimSpace(os.Getenv("AUTH_COOKIE_SECURE")); override != "" {
		if secure, err := strconv.ParseBool(override); err == nil {
			return secure
		}
		log.Printf("WARNING: invalid AUTH_COOKIE_SECURE value %q, falling back to auto mode", override)
	}

	if r == nil {
		return true
	}

	if r.TLS != nil {
		return true
	}

	forwardedProto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if strings.EqualFold(forwardedProto, "https") {
		return true
	}

	host := normalizeHost(r.Host)
	if host == "localhost" {
		return false
	}

	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return false
	}

	return true
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(parsedHost, "[]")
	}

	return strings.Trim(host, "[]")
}
