package services
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/linkmeAman/universal-middleware/test/integration"
	"github.com/stretchr/testify/suite"
)

type CommandServiceSuite struct {
	integration.IntegrationSuite
}

func TestCommandService(t *testing.T) {
	integration.RunIntegrationTest(t, new(CommandServiceSuite))
}

func (s *CommandServiceSuite) TestCommandProcessing() {
	// Test data
	command := map[string]interface{}{
		"type": "create_user",
		"payload": map[string]interface{}{
			"username": "newuser",
			"email":    "newuser@example.com",
		},
	}

	// Convert command to JSON
	body, err := json.Marshal(command)
	s.Require().NoError(err)

	// Send command
	resp, err := http.Post(
		fmt.Sprintf("%s/v1/commands", s.CommandServiceURL),
		"application/json",
		bytes.NewBuffer(body),
	)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusAccepted, resp.StatusCode)

	// Parse response
	var result struct {
		CommandID string `json:"command_id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	s.Require().NoError(err)
	resp.Body.Close()

	// Wait for command processing
	s.waitForCommandProcessing(result.CommandID)
}

func (s *CommandServiceSuite) waitForCommandProcessing(commandID string) {
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(
			fmt.Sprintf("%s/v1/commands/%s", s.CommandServiceURL, commandID),
		)
		if err == nil {
			var status struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&status); err == nil {
				resp.Body.Close()
				if status.Status == "completed" {
					return
				}
			}
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.T().Fatalf("Command %s not processed within timeout", commandID)
}