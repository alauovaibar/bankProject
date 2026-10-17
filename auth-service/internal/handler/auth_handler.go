package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auth-service/internal/appErr"
	"auth-service/internal/dto"
	"auth-service/internal/service"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr.WriteError(w, appErr.NewAppError(err, "invalid json body", http.StatusBadRequest))
		return
	}

	if req.Email == "" || req.Password == "" {
		appErr.WriteError(w, appErr.NewAppError(errors.New("missing fields"), "email and password are required", http.StatusBadRequest))
		return
	}

	user, err := h.authService.Register(r.Context(), req)
	if err != nil {
		appErr.WriteError(w, appErr.NewAppError(err, "registration failed: "+err.Error(), http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr.WriteError(w, appErr.NewAppError(err, "invalid json body", http.StatusBadRequest))
		return
	}

	tokens, err := h.authService.Login(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			appErr.WriteError(w, appErr.NewAppError(err, err.Error(), http.StatusUnauthorized))
			return
		}
		appErr.WriteError(w, appErr.NewAppError(err, "login failed", http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tokens)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appErr.WriteError(w, appErr.NewAppError(err, "invalid json body", http.StatusBadRequest))
		return
	}

	tokens, err := h.authService.Refresh(r.Context(), req)
	if err != nil {
		appErr.WriteError(w, appErr.NewAppError(err, "invalid refresh token", http.StatusUnauthorized))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tokens)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token := ""
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}

	userID := r.Header.Get("X-User-ID")
	_ = h.authService.Logout(r.Context(), token, userID)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"logged out successfully"}`))
}
