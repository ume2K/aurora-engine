package framework

import (
	"context"
	"net/http"
	"strings"
)

type AuthInfo struct {
	UserID string
	Role   string
}

type TokenVerifier func(rawToken string) (AuthInfo, error)

type authContextKey string

const (
	authUserIDKey authContextKey = "framework_auth_user_id"
	authRoleKey   authContextKey = "framework_auth_role"
)

func RequireBearerAuth(verifier TokenVerifier) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}

			info, err := verifier(strings.TrimSpace(parts[1]))
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), authUserIDKey, info.UserID)
			ctx = context.WithValue(ctx, authRoleKey, info.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AuthUserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(authUserIDKey).(string)
	return v, ok
}

func AuthRoleFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(authRoleKey).(string)
	return v, ok
}
