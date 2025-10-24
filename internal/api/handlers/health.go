package handlers

import (
	"encoding/json"
	"net/http"
)

// HealthResponse represents the health check response structure
type HealthResponse struct {
	Status   string            `json:"status"`
	Version  string            `json:"version"`
	Services map[string]string `json:"services,omitempty"`
}

// HealthHandler returns a handler function for health check endpoint
func HealthHandler(version string, dependencies map[string]func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "healthy"
		services := make(map[string]string)

		// Check all dependencies
		for name, check := range dependencies {
			if err := check(); err != nil {
				status = "degraded"
				services[name] = "unhealthy: " + err.Error()
			} else {
				services[name] = "healthy"
			}
		}

		response := HealthResponse{
			Status:   status,
			Version:  version,
			Services: services,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
