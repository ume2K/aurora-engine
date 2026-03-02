package video

import (
	"errors"
	"gocore/pkg/framework"
	"io"
	"net/http"
	"strconv"
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

	page, err := parsePositiveInt(c.Query("page"), 1)
	if err != nil {
		c.ErrorJSON(http.StatusBadRequest, "invalid page")
		return
	}
	limit, err := parsePositiveInt(c.Query("limit"), 20)
	if err != nil {
		c.ErrorJSON(http.StatusBadRequest, "invalid limit")
		return
	}
	query := ListQuery{
		Page:   page,
		Limit:  limit,
		Status: c.Query("status"),
		Q:      c.Query("q"),
	}

	videos, err := h.service.ListByOwnerWithQuery(c.R.Context(), ownerID, query)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			c.ErrorJSON(http.StatusBadRequest, "invalid list filters")
			return
		}
		c.ErrorJSON(http.StatusInternalServerError, "list videos failed")
		return
	}
	c.JSONSafe(http.StatusOK, map[string]any{
		"videos": videos,
		"pagination": map[string]int{
			"page":  page,
			"limit": limit,
		},
	})
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

func (h *Handler) Upload(c *framework.Context) {
	ownerID, ok := framework.AuthUserIDFromContext(c.R.Context())
	if !ok {
		c.ErrorJSON(http.StatusUnauthorized, "missing auth context")
		return
	}

	mr, err := c.R.MultipartReader()
	if err != nil {
		c.ErrorJSON(http.StatusBadRequest, "request must be multipart/form-data")
		return
	}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			c.ErrorJSON(http.StatusBadRequest, "missing required 'file' field")
			return
		}
		if err != nil {
			c.ErrorJSON(http.StatusBadRequest, "error reading multipart stream")
			return
		}

		if part.FormName() != "file" {
			part.Close()
			continue
		}

		filename := part.FileName()
		if filename == "" {
			part.Close()
			c.ErrorJSON(http.StatusBadRequest, "file part must include a filename")
			return
		}

		contentType := part.Header.Get("Content-Type")

		v, uploadErr := h.service.Upload(c.R.Context(), ownerID, part, filename, contentType)
		part.Close()

		if uploadErr != nil {
			if errors.Is(uploadErr, ErrInvalidInput) {
				c.ErrorJSON(http.StatusBadRequest, "invalid upload parameters")
				return
			}
			c.ErrorJSON(http.StatusInternalServerError, "upload failed")
			return
		}

		c.JSONSafe(http.StatusCreated, map[string]any{"video": v})
		return
	}
}

func parsePositiveInt(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, errors.New("invalid positive int")
	}
	return n, nil
}
