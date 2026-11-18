package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"eneba-backend/internal/httpx"
	"eneba-backend/internal/responses"
)

// *pgxpool.Pool satisfies Pinger, and a test can pass a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	db Pinger
}

func NewHealthHandler(db Pinger) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		slog.WarnContext(r.Context(), "health check: database unreachable", "error", err)
		httpx.WriteJSON(w, http.StatusServiceUnavailable, responses.HealthResponse{Status: "degraded", Database: "down"})
		return
	}

	httpx.WriteJSON(w, http.StatusOK, responses.HealthResponse{Status: "ok", Database: "up"})
}
