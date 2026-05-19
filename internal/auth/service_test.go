package auth

import (
	"context"
	"testing"
	"time"
)

type authRepoStub struct {
	createFn       func(ctx context.Context, email, passwordHash string) (User, error)
	getFn          func(ctx context.Context, email string) (User, error)
	storedTokens   map[string]string // tokenHash -> userID
	consumedTokens []string
}

func newRepoStub() *authRepoStub {
	return &authRepoStub{
		storedTokens: make(map[string]string),
		createFn: func(ctx context.Context, email, passwordHash string) (User, error) {
			return User{ID: "u-1", Email: email, Role: "user"}, nil
		},
		getFn: func(ctx context.Context, email string) (User, error) { return User{}, nil },
	}
}

func (s *authRepoStub) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	return s.createFn(ctx, email, passwordHash)
}

func (s *authRepoStub) GetUserByEmail(ctx context.Context, email string) (User, error) {
	return s.getFn(ctx, email)
}

func (s *authRepoStub) GetUserByID(ctx context.Context, id string) (User, error) {
	return User{ID: id, Email: "test@example.com", Role: "user"}, nil
}

func (s *authRepoStub) StoreRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	s.storedTokens[tokenHash] = userID
	return nil
}

func (s *authRepoStub) ConsumeRefreshToken(ctx context.Context, tokenHash string) (string, string, error) {
	userID, ok := s.storedTokens[tokenHash]
	if !ok {
		return "", "", ErrInvalidToken
	}
	delete(s.storedTokens, tokenHash)
	s.consumedTokens = append(s.consumedTokens, tokenHash)
	return userID, "user", nil
}

func (s *authRepoStub) DeleteRefreshTokensByUser(ctx context.Context, userID string) error {
	for k, v := range s.storedTokens {
		if v == userID {
			delete(s.storedTokens, k)
		}
	}
	return nil
}

func newTestService(repo *authRepoStub) *Service {
	return NewService(repo, NewJWTManager("test-secret", time.Hour), 7*24*time.Hour)
}

func TestRegister_HappyPath(t *testing.T) {
	repo := newRepoStub()
	repo.createFn = func(ctx context.Context, email, passwordHash string) (User, error) {
		if email != "test@example.com" {
			t.Fatalf("unexpected email: %s", email)
		}
		if passwordHash == "" || passwordHash == "Password123!" {
			t.Fatalf("expected hashed password, got: %s", passwordHash)
		}
		return User{ID: "u-1", Email: email, Role: "user"}, nil
	}
	svc := newTestService(repo)

	user, pair, err := svc.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected user ID")
	}
	if pair.AccessToken == "" {
		t.Fatal("expected access token")
	}
	if pair.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}
	if pair.ExpiresIn <= 0 {
		t.Fatalf("expected positive expires_in, got %d", pair.ExpiresIn)
	}
	if len(repo.storedTokens) != 1 {
		t.Fatalf("expected 1 stored refresh token, got %d", len(repo.storedTokens))
	}
}

func TestRegister_InvalidInput(t *testing.T) {
	svc := newTestService(newRepoStub())

	_, _, err := svc.Register(context.Background(), RegisterInput{
		Email:    "bad-email",
		Password: "short",
	})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}

func TestRefreshTokens_HappyPath(t *testing.T) {
	repo := newRepoStub()
	svc := newTestService(repo)

	_, pair, err := svc.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	newPair, err := svc.RefreshTokens(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh returned error: %v", err)
	}
	if newPair.AccessToken == "" || newPair.RefreshToken == "" {
		t.Fatal("expected new token pair")
	}
	if newPair.RefreshToken == pair.RefreshToken {
		t.Fatal("refresh token should have been rotated")
	}
}

func TestRefreshTokens_InvalidToken(t *testing.T) {
	svc := newTestService(newRepoStub())

	_, err := svc.RefreshTokens(context.Background(), "nonexistent-token")
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got: %v", err)
	}
}

func TestRefreshTokens_ReplayPrevented(t *testing.T) {
	repo := newRepoStub()
	svc := newTestService(repo)

	_, pair, err := svc.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err = svc.RefreshTokens(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	_, err = svc.RefreshTokens(context.Background(), pair.RefreshToken)
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken on replay, got: %v", err)
	}
}

func TestLogout(t *testing.T) {
	repo := newRepoStub()
	svc := newTestService(repo)

	_, pair, err := svc.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	err = svc.Logout(context.Background(), pair.RefreshToken)
	if err != nil {
		t.Fatalf("logout returned error: %v", err)
	}

	if len(repo.storedTokens) != 0 {
		t.Fatalf("expected 0 stored tokens after logout, got %d", len(repo.storedTokens))
	}

	_, err = svc.RefreshTokens(context.Background(), pair.RefreshToken)
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken after logout, got: %v", err)
	}
}
