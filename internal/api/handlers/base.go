package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
)

// Handler encapsulates dependencies for API handlers
type Handler struct {
	log     *logger.Logger
	metrics *metrics.Metrics
}

// NewHandler creates a new Handler instance
func NewHandler(log *logger.Logger, m *metrics.Metrics) *Handler {
	return &Handler{
		log:     log,
		metrics: m,
	}
}

// respondJSON sends a JSON response with the given status code
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			h.log.Error("Failed to encode JSON response",
				zap.Error(err))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

// respondError sends an error response
func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{
		"error": message,
	})
}
