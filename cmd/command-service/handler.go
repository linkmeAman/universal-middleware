package main

import (
	"encoding/json"
	"net/http"

	"github.com/linkmeAman/universal-middleware/internal/command"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

// CommandRequest represents an incoming command request
type CommandRequest struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// HandleCommand creates a handler for processing commands
func HandleCommand(processor *command.Processor, log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CommandRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Create command
		cmd := command.NewCommand(req.Type, req.Payload)

		// Process command
		if err := processor.Process(r.Context(), cmd); err != nil {
			log.Error("Failed to process command", zap.Error(err))
			http.Error(w, "Command processing failed", http.StatusInternalServerError)
			return
		}

		// Return command ID
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"command_id": cmd.ID,
		})
	}
}
