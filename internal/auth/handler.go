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
		if err := c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()}); err != nil {
			return
		}
		return
	}

	user, token, err := h.service.Register(c.R.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			_ = c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid email or password (min 8 chars)"})
		case errors.Is(err, ErrEmailAlreadyExists):
			_ = c.JSON(http.StatusConflict, map[string]string{"error": "email already exists"})
		default:
			_ = c.JSON(http.StatusInternalServerError, map[string]string{"error": "register failed"})
		}
		return
	}

	if err := c.JSON(http.StatusCreated, map[string]any{
		"user":  user,
		"token": token,
	}); err != nil {
		return
	}
}

func (h *Handler) Login(c *framework.Context) {
	var input LoginInput
	if err := c.BindJSONStrict(&input); err != nil {
		_ = c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	user, token, err := h.service.Login(c.R.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			_ = c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid email or password"})
		case errors.Is(err, ErrInvalidCredentials):
			_ = c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		default:
			_ = c.JSON(http.StatusInternalServerError, map[string]string{"error": "login failed"})
		}
		return
	}

	if err := c.JSON(http.StatusOK, map[string]any{
		"user":  user,
		"token": token,
	}); err != nil {
		return
	}
}

func (h *Handler) Me(c *framework.Context) {
	userID, ok := framework.AuthUserIDFromContext(c.R.Context())
	if !ok {
		_ = c.JSON(http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
		return
	}

	user, err := h.service.GetCurrentUser(c.R.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			_ = c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		default:
			_ = c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch user"})
		}
		return
	}
	_ = c.JSON(http.StatusOK, map[string]any{"user": user})
}
