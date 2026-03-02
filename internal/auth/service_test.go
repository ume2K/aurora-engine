package auth

import (
	"context"
	"testing"
	"time"
)

type authRepoStub struct {
	createFn func(ctx context.Context, email, passwordHash string) (User, error)
	getFn    func(ctx context.Context, email string) (User, error)
}

func (s authRepoStub) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	return s.createFn(ctx, email, passwordHash)
}

func (s authRepoStub) GetUserByEmail(ctx context.Context, email string) (User, error) {
	return s.getFn(ctx, email)
}

func (s authRepoStub) GetUserByID(ctx context.Context, id string) (User, error) {
	return User{}, nil
}

func TestRegister_HappyPath(t *testing.T) {
	repo := authRepoStub{
		createFn: func(ctx context.Context, email, passwordHash string) (User, error) {
			if email != "test@example.com" {
				t.Fatalf("unexpected email: %s", email)
			}
			if passwordHash == "" || passwordHash == "Password123!" {
				t.Fatalf("expected hashed password, got: %s", passwordHash)
			}
			return User{ID: "u-1", Email: email, Role: "user"}, nil
		},
		getFn: func(ctx context.Context, email string) (User, error) { return User{}, nil },
	}
	svc := NewService(repo, NewJWTManager("test-secret", time.Hour))

	user, token, err := svc.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123!",
	})
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	if user.ID == "" || token == "" {
		t.Fatalf("expected user and token, got user=%+v token=%q", user, token)
	}
}

func TestRegister_InvalidInput(t *testing.T) {
	svc := NewService(authRepoStub{
		createFn: func(ctx context.Context, email, passwordHash string) (User, error) { return User{}, nil },
		getFn:    func(ctx context.Context, email string) (User, error) { return User{}, nil },
	}, NewJWTManager("test-secret", time.Hour))

	_, _, err := svc.Register(context.Background(), RegisterInput{
		Email:    "bad-email",
		Password: "short",
	})
	if err != ErrInvalidInput {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}
}
