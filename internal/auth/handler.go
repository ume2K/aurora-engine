package auth

import (
	"errors"
	"gocore/pkg/framework"
	"net/http"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(c *framework.Context) {
	var input RegisterInput
	if err := c.BindJSONStrict(&input); err != nil {
		c.ErrorJSON(http.StatusBadRequest, err.Error())
		return
	}

	user, pair, err := h.service.Register(c.R.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.ErrorJSON(http.StatusBadRequest, "invalid email or password (min 8 chars)")
		case errors.Is(err, ErrEmailAlreadyExists):
			c.ErrorJSON(http.StatusConflict, "email already exists")
		default:
			c.ErrorJSON(http.StatusInternalServerError, "register failed")
		}
		return
	}

	c.JSONSafe(http.StatusCreated, map[string]any{
		"user":          user,
		"token":         pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
	})
}

func (h *Handler) Login(c *framework.Context) {
	var input LoginInput
	if err := c.BindJSONStrict(&input); err != nil {
		c.ErrorJSON(http.StatusBadRequest, err.Error())
		return
	}

	user, pair, err := h.service.Login(c.R.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.ErrorJSON(http.StatusBadRequest, "invalid email or password")
		case errors.Is(err, ErrInvalidCredentials):
			c.ErrorJSON(http.StatusUnauthorized, "invalid credentials")
		default:
			c.ErrorJSON(http.StatusInternalServerError, "login failed")
		}
		return
	}

	c.JSONSafe(http.StatusOK, map[string]any{
		"user":          user,
		"token":         pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
	})
}

func (h *Handler) Refresh(c *framework.Context) {
	var input RefreshInput
	if err := c.BindJSONStrict(&input); err != nil {
		c.ErrorJSON(http.StatusBadRequest, err.Error())
		return
	}

	pair, err := h.service.RefreshTokens(c.R.Context(), input.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			c.ErrorJSON(http.StatusUnauthorized, "invalid or expired refresh token")
			return
		}
		c.ErrorJSON(http.StatusInternalServerError, "refresh failed")
		return
	}

	c.JSONSafe(http.StatusOK, map[string]any{
		"token":         pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
	})
}

func (h *Handler) Logout(c *framework.Context) {
	var input RefreshInput
	if err := c.BindJSONStrict(&input); err != nil {
		c.ErrorJSON(http.StatusBadRequest, err.Error())
		return
	}

	if err := h.service.Logout(c.R.Context(), input.RefreshToken); err != nil {
		c.ErrorJSON(http.StatusInternalServerError, "logout failed")
		return
	}

	c.W.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(c *framework.Context) {
	userID, ok := framework.AuthUserIDFromContext(c.R.Context())
	if !ok {
		c.ErrorJSON(http.StatusUnauthorized, "missing auth context")
		return
	}

	user, err := h.service.GetCurrentUser(c.R.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.ErrorJSON(http.StatusNotFound, "user not found")
		default:
			c.ErrorJSON(http.StatusInternalServerError, "failed to fetch user")
		}
		return
	}
	c.JSONSafe(http.StatusOK, map[string]any{"user": user})
}
