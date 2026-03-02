package video

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

func (h *Handler) Create(c *framework.Context) {
	ownerID, ok := framework.AuthUserIDFromContext(c.R.Context())
	if !ok {
		c.ErrorJSON(http.StatusUnauthorized, "missing auth context")
		return
	}

	var input CreateInput
	if err := c.BindJSONStrict(&input); err != nil {
		c.ErrorJSON(http.StatusBadRequest, err.Error())
		return
	}

	v, err := h.service.Create(c.R.Context(), ownerID, input)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.ErrorJSON(http.StatusBadRequest, "invalid video metadata")
			return
		}
		c.ErrorJSON(http.StatusInternalServerError, "create video failed")
		return
	}
	c.JSONSafe(http.StatusCreated, map[string]any{"video": v})
}

func (h *Handler) List(c *framework.Context) {
	ownerID, ok := framework.AuthUserIDFromContext(c.R.Context())
	if !ok {
		c.ErrorJSON(http.StatusUnauthorized, "missing auth context")
		return
	}

	videos, err := h.service.ListByOwner(c.R.Context(), ownerID)
	if err != nil {
		c.ErrorJSON(http.StatusInternalServerError, "list videos failed")
		return
	}
	c.JSONSafe(http.StatusOK, map[string]any{"videos": videos})
}

func (h *Handler) Get(c *framework.Context) {
	ownerID, ok := framework.AuthUserIDFromContext(c.R.Context())
	if !ok {
		c.ErrorJSON(http.StatusUnauthorized, "missing auth context")
		return
	}

	videoID := c.Param("id")
	v, err := h.service.GetByID(c.R.Context(), ownerID, videoID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.ErrorJSON(http.StatusBadRequest, "invalid video id")
		case errors.Is(err, ErrVideoNotFound):
			c.ErrorJSON(http.StatusNotFound, "video not found")
		default:
			c.ErrorJSON(http.StatusInternalServerError, "get video failed")
		}
		return
	}
	c.JSONSafe(http.StatusOK, map[string]any{"video": v})
}

func (h *Handler) Update(c *framework.Context) {
	ownerID, ok := framework.AuthUserIDFromContext(c.R.Context())
	if !ok {
		c.ErrorJSON(http.StatusUnauthorized, "missing auth context")
		return
	}

	videoID := c.Param("id")
	var input UpdateInput
	if err := c.BindJSONStrict(&input); err != nil {
		c.ErrorJSON(http.StatusBadRequest, err.Error())
		return
	}

	v, err := h.service.Update(c.R.Context(), ownerID, videoID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.ErrorJSON(http.StatusBadRequest, "invalid update payload")
		case errors.Is(err, ErrVideoNotFound):
			c.ErrorJSON(http.StatusNotFound, "video not found")
		default:
			c.ErrorJSON(http.StatusInternalServerError, "update video failed")
		}
		return
	}
	c.JSONSafe(http.StatusOK, map[string]any{"video": v})
}

func (h *Handler) Delete(c *framework.Context) {
	ownerID, ok := framework.AuthUserIDFromContext(c.R.Context())
	if !ok {
		c.ErrorJSON(http.StatusUnauthorized, "missing auth context")
		return
	}

	videoID := c.Param("id")
	err := h.service.Delete(c.R.Context(), ownerID, videoID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.ErrorJSON(http.StatusBadRequest, "invalid video id")
		case errors.Is(err, ErrVideoNotFound):
			c.ErrorJSON(http.StatusNotFound, "video not found")
		default:
			c.ErrorJSON(http.StatusInternalServerError, "delete video failed")
		}
		return
	}
	c.JSONSafe(http.StatusOK, map[string]string{"status": "deleted"})
}
