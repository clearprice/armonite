package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

type TestPlan struct {
	Name           string          `yaml:"name" json:"name"`
	Duration       string          `yaml:"duration" json:"duration"`
	Concurrency    int             `yaml:"concurrency" json:"concurrency"`
	RampUp         string          `yaml:"ramp_up,omitempty" json:"ramp_up,omitempty"`                   // Legacy field for backwards compatibility
	RampUpStrategy *RampUpStrategy `yaml:"ramp_up_strategy,omitempty" json:"ramp_up_strategy,omitempty"` // New structured ramp-up
	Endpoints      []Endpoint      `yaml:"endpoints" json:"endpoints"`
}

type Endpoint struct {
	Method    string                 `yaml:"method"`
	URL       string                 `yaml:"url"`
	Headers   map[string]string      `yaml:"headers"`
	Body      map[string]interface{} `yaml:"body"`
	ThinkTime string                 `yaml:"think_time"`
}

type Coordinator struct {
	natsServer        *server.Server
	natsConn          *nats.Conn
	port              int
	host              string
	config            *Config
	database          *Database
	connectedAgents   map[string]*AgentInfo
	testRuns          map[string]*TestRun
	currentTestRun    *TestRun
	agentResults      map[string][]AgentResult // keyed by test run ID
	phaseOrchestrator *PhaseOrchestrator       // For coordinated phase execution
	mu                sync.RWMutex
}

type AgentInfo struct {
	ID              string
	Region          string
	ConnectedAt     time.Time
	LastSeen        time.Time
	Concurrency     int
	ExecutionState  string
	RampUpExecution *RampUpExecution // Current ramp-up state
}

func runCoordinator(cmd *cobra.Command, args []string) error {
	config := globalConfig

	// Override config with CLI flags if provided
	if host, _ := cmd.Flags().GetString("host"); host != "" {
		config.Server.Host = host
	}
	if port, _ := cmd.Flags().GetInt("port"); port != 0 {
		config.Server.Port = port
	}
	if httpPort, _ := cmd.Flags().GetInt("http-port"); httpPort != 0 {
		config.Server.HTTPPort = httpPort
	}
	if minAgents, _ := cmd.Flags().GetInt("min-agents"); minAgents != 0 {
		config.Defaults.MinAgents = minAgents
	}
	if enableUI, _ := cmd.Flags().GetBool("ui"); enableUI {
		config.Server.EnableUI = true
	}

	coordinator := &Coordinator{
		port:            config.Server.Port,
		host:            config.Server.Host,
		config:          config,
		connectedAgents: make(map[string]*AgentInfo),
		testRuns:        make(map[string]*TestRun),
		agentResults:    make(map[string][]AgentResult),
	}

	// Initialize database
	db, err := NewDatabase(config.Database)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	coordinator.database = db

	// Load existing test runs from database
	if err := coordinator.loadTestRunsFromDatabase(); err != nil {
		LogWarn("Failed to load test runs from database: %v", err)
	}

	if err := coordinator.startNATSServer(); err != nil {
		return fmt.Errorf("failed to start NATS server: %w", err)
	}
	defer coordinator.shutdown()

	if err := coordinator.connectToNATS(); err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	coordinator.startAgentRegistration()
	coordinator.startInternalMessageHandler() // Start internal message handler first
	coordinator.startTelemetryCollection()
	coordinator.startStatusDisplay()
	coordinator.startHTTPServer()

	// Print startup banner with ASCII art
	printStartupBanner(config)

	LogInfo("Coordinator ready - waiting for test plans via HTTP API")
	LogInfo("API server: http://%s:%d", config.Server.Host, config.Server.HTTPPort)
	LogInfo("Create test plans: POST /api/v1/test-runs")
	LogInfo("Start tests: POST /api/v1/test-runs/{id}/start")
	if config.Server.EnableUI {
		uiPort := config.Server.HTTPPort + 1
		if uiPort == 1 {
			uiPort = 8081
		}
		LogInfo("Web UI available at: http://%s:%d/ui", config.Server.Host, uiPort)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	LogInfo("Received shutdown signal, shutting down coordinator...")
	coordinator.shutdown()
	return nil
}

func (c *Coordinator) startNATSServer() error {
	opts := &server.Options{
		Host: c.host,
		Port: c.port,
	}

	LogDebug("Starting NATS server on %s:%d", c.host, c.port)

	var err error
	c.natsServer, err = server.NewServer(opts)
	if err != nil {
		return fmt.Errorf("failed to create NATS server: %w", err)
	}

	go c.natsServer.Start()

	// Wait for server to be ready with better error reporting
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("NATS server failed to start within 10 seconds")
		case <-ticker.C:
			LogDebug("Checking if NATS server is ready...")
			if c.natsServer.ReadyForConnections(100 * time.Millisecond) {
				LogInfo("NATS server started successfully on %s:%d", c.host, c.port)
				return nil
			}
		}
	}
}

func (c *Coordinator) connectToNATS() error {
	var err error
	c.natsConn, err = nats.Connect(fmt.Sprintf("nats://%s:%d", c.host, c.port))
	return err
}

func (c *Coordinator) startTelemetryCollection() {
	// Use dedicated goroutine for telemetry processing
	go func() {
		// Rate limiting state - local to this goroutine
		lastUpdate := make(map[string]time.Time)

		// Subscribe to telemetry messages
		_, err := c.natsConn.Subscribe("armonite.telemetry", func(msg *nats.Msg) {
			var metrics AgentMetrics
			if err := json.Unmarshal(msg.Data, &metrics); err != nil {
				LogError("Failed to unmarshal telemetry: %v", err)
				return
			}

			// Rate limiting: only process updates from same agent if >1 second has passed
			now := time.Now()
			if lastTime, exists := lastUpdate[metrics.AgentID]; exists && now.Sub(lastTime) < time.Second {
				return // Skip this update to prevent flooding
			}
			lastUpdate[metrics.AgentID] = now

			LogDebug("Processing telemetry from agent %s: requests=%d, errors=%d, avg_latency=%.2fms",
				metrics.AgentID, metrics.Requests, metrics.Errors, metrics.AvgLatencyMs)

			// Send telemetry update via NATS to internal handler
			c.handleTelemetryUpdate(metrics)
		})

		if err != nil {
			LogError("Failed to subscribe to telemetry: %v", err)
			return
		}

		// Keep this goroutine alive
		select {}
	}()
}

// Removed old broadcasting functions - replaced with workflow-based approach

func (c *Coordinator) shutdown() {
	LogInfo("Shutting down coordinator...")

	// Gracefully shutdown NATS connection first
	if c.natsConn != nil {
		c.natsConn.Drain()
		c.natsConn.Close()
		c.natsConn = nil
	}

	// Stop phase orchestrator if running
	c.mu.Lock()
	if c.phaseOrchestrator != nil {
		c.phaseOrchestrator.Stop()
		c.phaseOrchestrator = nil
	}
	c.mu.Unlock()

	// Shutdown NATS server gracefully
	if c.natsServer != nil {
		// Wait a bit for connections to drain
		time.Sleep(100 * time.Millisecond)
		c.natsServer.Shutdown()
		c.natsServer.WaitForShutdown()
		c.natsServer = nil
	}

	// Close database last
	if c.database != nil {
		c.database.Close()
		c.database = nil
	}

	LogInfo("Coordinator shutdown complete")
}

func (c *Coordinator) loadTestRunsFromDatabase() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	testRuns, err := c.database.ListTestRuns(100, 0) // Load last 100 test runs
	if err != nil {
		return err
	}

	for _, testRun := range testRuns {
		c.testRuns[testRun.ID] = testRun

		// If we find a running test run, set it as current
		if testRun.Status == TestRunStatusRunning || testRun.Status == TestRunStatusWaiting {
			c.currentTestRun = testRun

			// Load agent results for the current test run
			if agentResults, err := c.database.GetAgentResults(testRun.ID); err == nil {
				c.agentResults[testRun.ID] = agentResults
			}
		}
	}

	LogInfo("Loaded %d test runs from database", len(testRuns))
	return nil
}
