package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUserNotFound       = errors.New("user not found")
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type Service struct {
	repo       Repository
	jwt        *JWTManager
	refreshTTL time.Duration
}

func NewService(repo Repository, jwt *JWTManager, refreshTTL time.Duration) *Service {
	return &Service{repo: repo, jwt: jwt, refreshTTL: refreshTTL}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, TokenPair, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)

	if !emailPattern.MatchString(email) || len(password) < 8 {
		return User{}, TokenPair{}, ErrInvalidInput
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, email, string(hashBytes))
	if err != nil {
		return User{}, TokenPair{}, err
	}

	pair, err := s.generateTokenPair(ctx, user.ID, user.Role)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	user.PasswordHash = ""
	return user, pair, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (User, TokenPair, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)
	if !emailPattern.MatchString(email) || password == "" {
		return User{}, TokenPair{}, ErrInvalidInput
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return User{}, TokenPair{}, ErrInvalidCredentials
	}

	pair, err := s.generateTokenPair(ctx, user.ID, user.Role)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	user.PasswordHash = ""
	return user, pair, nil
}

func (s *Service) RefreshTokens(ctx context.Context, rawRefreshToken string) (TokenPair, error) {
	if rawRefreshToken == "" {
		return TokenPair{}, ErrInvalidToken
	}

	hash := hashToken(rawRefreshToken)
	userID, role, err := s.repo.ConsumeRefreshToken(ctx, hash)
	if err != nil {
		return TokenPair{}, err
	}

	return s.generateTokenPair(ctx, userID, role)
}

func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	hash := hashToken(rawRefreshToken)
	_, _, err := s.repo.ConsumeRefreshToken(ctx, hash)
	if err != nil && !errors.Is(err, ErrInvalidToken) {
		return err
	}
	return nil
}

func (s *Service) GetCurrentUser(ctx context.Context, userID string) (User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return User{}, err
	}
	user.PasswordHash = ""
	return user, nil
}

func (s *Service) generateTokenPair(ctx context.Context, userID, role string) (TokenPair, error) {
	accessToken, err := s.jwt.CreateToken(userID, role)
	if err != nil {
		return TokenPair{}, fmt.Errorf("create access token: %w", err)
	}

	rawRefresh, err := generateOpaqueToken()
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	hash := hashToken(rawRefresh)
	expiresAt := time.Now().UTC().Add(s.refreshTTL)
	if err := s.repo.StoreRefreshToken(ctx, userID, hash, expiresAt); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int(s.jwt.ttl.Seconds()),
	}, nil
}

func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
