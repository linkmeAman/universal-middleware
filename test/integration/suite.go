package integration
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/linkmeAman/universal-middleware/pkg/config"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/stretchr/testify/suite"
)

// IntegrationSuite is the base suite for all integration tests
type IntegrationSuite struct {
	suite.Suite
	Config *config.Config
	DB     *sql.DB
	Log    *logger.Logger

	// Service URLs
	APIGatewayURL    string
	WSHubURL         string
	CommandServiceURL string
	ProcessorURL     string
	CacheUpdaterURL  string
}

// SetupSuite prepares the test environment
func (s *IntegrationSuite) SetupSuite() {
	var err error

	// Load test configuration
	s.Config, err = config.Load()
	s.Require().NoError(err, "Failed to load config")

	// Initialize logger
	s.Log, err = logger.New("test", "debug")
	s.Require().NoError(err, "Failed to initialize logger")

	// Set service URLs
	s.APIGatewayURL = "http://localhost:8080"
	s.WSHubURL = "http://localhost:8081"
	s.CommandServiceURL = "http://localhost:8082"
	s.ProcessorURL = "http://localhost:8083"
	s.CacheUpdaterURL = "http://localhost:8084"

	// Wait for services to be ready
	s.waitForServices()
}

// TearDownSuite cleans up test resources
func (s *IntegrationSuite) TearDownSuite() {
	if s.DB != nil {
		s.DB.Close()
	}
}

// waitForServices ensures all services are healthy before running tests
func (s *IntegrationSuite) waitForServices() {
	services := map[string]string{
		"API Gateway":     s.APIGatewayURL,
		"WS Hub":         s.WSHubURL,
		"Command":        s.CommandServiceURL,
		"Processor":      s.ProcessorURL,
		"Cache Updater":  s.CacheUpdaterURL,
	}

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	for name, url := range services {
		deadline := time.Now().Add(30 * time.Second)
		for {
			resp, err := client.Get(fmt.Sprintf("%s/health", url))
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				break
			}
			if time.Now().After(deadline) {
				s.T().Fatalf("Service %s not healthy after 30 seconds", name)
			}
			time.Sleep(time.Second)
		}
	}
}

// RunIntegrationTest runs the integration test suite
func RunIntegrationTest(t *testing.T, s suite.TestingSuite) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	suite.Run(t, s)
}