package http

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"master/internal/db"
	"github.com/golang-jwt/jwt/v5"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	userDB    *db.UserDB
	jwtSecret []byte
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(userDB *db.UserDB) *AuthHandler {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "vishvboda"
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

	if len(req.Password) < 6 {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: "Password must be at least 6 characters",
		})
		return
	}

	// Create user
	err := h.userDB.CreateUser(req.Name, req.Email, req.Password)
	if err != nil {
		json.NewEncoder(w).Encode(AuthResponse{
			Success: false,
			Message: err.Error(),
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
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    tokenString,
		Expires:  expirationTime,
		Path:     "/",
		HttpOnly: true,
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
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		Path:     "/",
		HttpOnly: true,
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
	email, ok := r.Context().Value("user_email").(string)
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
