package framework

import (
	"context"
	"encoding/json"
	"log"
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
				writeErrorJSON(w, http.StatusUnauthorized, "missing bearer token")
				return
			}

			info, err := verifier(strings.TrimSpace(parts[1]))
			if err != nil {
				writeErrorJSON(w, http.StatusUnauthorized, "invalid token")
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

func writeErrorJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	payload := map[string]any{
		"error": map[string]string{
			"code":    strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_")),
			"message": message,
		},
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write auth error json failed (status=%d): %v", status, err)
	}
}
