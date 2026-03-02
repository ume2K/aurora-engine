package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

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
	repo Repository
	jwt  *JWTManager
}

func NewService(repo Repository, jwt *JWTManager) *Service {
	return &Service{repo: repo, jwt: jwt}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, string, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)

	if !emailPattern.MatchString(email) || len(password) < 8 {
		return User{}, "", ErrInvalidInput
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, "", fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, email, string(hashBytes))
	if err != nil {
		return User{}, "", err
	}

	token, err := s.jwt.CreateToken(user.ID, user.Role)
	if err != nil {
		return User{}, "", err
	}
	user.PasswordHash = ""
	return user, token, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (User, string, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)
	if !emailPattern.MatchString(email) || password == "" {
		return User{}, "", ErrInvalidInput
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return User{}, "", ErrInvalidCredentials
	}

	token, err := s.jwt.CreateToken(user.ID, user.Role)
	if err != nil {
		return User{}, "", err
	}
	user.PasswordHash = ""
	return user, token, nil
}

func (s *Service) GetCurrentUser(ctx context.Context, userID string) (User, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return User{}, err
	}
	user.PasswordHash = ""
	return user, nil
}
