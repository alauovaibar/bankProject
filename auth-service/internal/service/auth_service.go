package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"auth-service/internal/dto"
	"auth-service/internal/entities"
	"auth-service/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*entities.User, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error)
	Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.TokenResponse, error)
	Logout(ctx context.Context, token, userID string) error
}

type authService struct {
	userRepo    repository.UserRepository
	sessionRepo repository.SessionRepository
	jwtSecret   []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
}

func NewAuthService(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	jwtSecret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) AuthService {
	return &authService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtSecret:   []byte(jwtSecret),
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
	}
}

func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*entities.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &entities.User{
		Email:        req.Email,
		PasswordHash: string(hash),
	}

	err = s.userRepo.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.generateTokenPair(ctx, user.ID)
}

func (s *authService) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.TokenResponse, error) {
	// 1. Парсим refresh токен для извлечения userID
	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return nil, ErrInvalidToken
	}

	// 2. Проверяем валидность в Redis
	valid, err := s.sessionRepo.ValidateRefreshToken(ctx, userID, req.RefreshToken)
	if err != nil || !valid {
		return nil, ErrInvalidToken
	}

	// 3. Генерируем новую пару токенов
	return s.generateTokenPair(ctx, userID)
}

func (s *authService) Logout(ctx context.Context, token, userID string) error {
	_ = s.sessionRepo.RevokeRefreshToken(ctx, userID)
	if token != "" {
		_ = s.sessionRepo.AddToBlacklist(ctx, token, s.accessTTL)
	}
	return nil
}

func (s *authService) generateTokenPair(ctx context.Context, userID string) (*dto.TokenResponse, error) {
	// Access Token (JWT)
	now := time.Now()
	accessClaims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(s.accessTTL).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessString, err := accessToken.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	// Refresh Token (JWT с длительным сроком)
	refreshClaims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(s.refreshTTL).Unix(),
		"rnd": generateRandomHex(16),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshString, err := refreshToken.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}

	// Сохраняем в Redis
	err = s.sessionRepo.SaveRefreshToken(ctx, userID, refreshString, s.refreshTTL)
	if err != nil {
		return nil, err
	}

	return &dto.TokenResponse{
		AccessToken:  accessString,
		RefreshToken: refreshString,
	}, nil
}

func generateRandomHex(n int) string {
	bytes := make([]byte, n)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
