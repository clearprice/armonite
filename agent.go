package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
)

type Agent struct {
	id               string
	region           string
	masterHost       string
	masterPort       int
	concurrency      int
	keepAlive        bool
	natsConn         *nats.Conn
	httpClient       *http.Client
	currentPlan      *TestPlan
	currentTestRunID string
	metrics          *AgentMetrics
	running          bool
	testStarted      bool
	testCompleted    bool
	rampUpExecution  *RampUpExecution
	rampUpCalculator *RampUpCalculator

	// Phase execution state
	currentPhase *PhaseInfo
	phaseStopCh  chan struct{}

	// Development mode settings
	devMode          bool
	rateLimit        int           // Requests per second
	defaultThinkTime time.Duration // Default think time between requests
	rateLimiter      chan struct{} // Rate limiting channel

	mu sync.RWMutex
}

type AgentMetrics struct {
	AgentID      string           `json:"agent_id"`
	Timestamp    string           `json:"timestamp"`
	Requests     int64            `json:"requests"`
	Errors       int64            `json:"errors"`
	AvgLatencyMs float64          `json:"avg_latency_ms"`
	MinLatencyMs float64          `json:"min_latency_ms"`
	MaxLatencyMs float64          `json:"max_latency_ms"`
	StatusCodes  map[string]int64 `json:"status_codes"`
	mu           sync.Mutex
	totalLatency float64
}

func runAgent(cmd *cobra.Command, args []string) error {
	config := globalConfig

	// Override config with CLI flags if provided
	masterHost, _ := cmd.Flags().GetString("master-host")
	masterPort, _ := cmd.Flags().GetInt("master-port")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	keepAlive, _ := cmd.Flags().GetBool("keep-alive")
	region, _ := cmd.Flags().GetString("region")
	id, _ := cmd.Flags().GetString("id")
	devMode, _ := cmd.Flags().GetBool("dev")
	rateLimit, _ := cmd.Flags().GetInt("rate-limit")
	defaultThinkTimeStr, _ := cmd.Flags().GetString("default-think-time")

	if masterHost == "" {
		masterHost = config.Server.Host
	}
	if masterPort == 0 {
		masterPort = config.Server.Port
	}
	if concurrency == 0 {
		concurrency = config.Defaults.Concurrency
	}
	if !cmd.Flags().Changed("keep-alive") {
		keepAlive = config.Defaults.KeepAlive
	}

	// Apply development mode defaults
	var defaultThinkTime time.Duration
	if devMode {
		LogInfo("Development mode enabled - applying resource limits")

		// Set sensible defaults for development
		if concurrency == 0 || concurrency > 100 {
			concurrency = 100
			LogInfo("Dev mode: Limited concurrency to %d", concurrency)
		}

		if rateLimit == 0 {
			rateLimit = 1000
			LogInfo("Dev mode: Set rate limit to %d requests/second", rateLimit)
		}

		if defaultThinkTimeStr == "" {
			defaultThinkTime = 200 * time.Millisecond
			LogInfo("Dev mode: Set default think time to %s", defaultThinkTime)
		}
	}

	// Parse custom think time if provided
	if defaultThinkTimeStr != "" {
		var err error
		defaultThinkTime, err = time.ParseDuration(defaultThinkTimeStr)
		if err != nil {
			LogWarn("Invalid default-think-time '%s', ignoring: %v", defaultThinkTimeStr, err)
			defaultThinkTime = 0
		}
	}

	if id == "" {
		id = fmt.Sprintf("agent-%d", time.Now().Unix())
	}

	agent := &Agent{
		id:               id,
		region:           region,
		masterHost:       masterHost,
		masterPort:       masterPort,
		concurrency:      concurrency,
		keepAlive:        keepAlive,
		devMode:          devMode,
		rateLimit:        rateLimit,
		defaultThinkTime: defaultThinkTime,
		metrics: &AgentMetrics{
			AgentID:     id,
			StatusCodes: make(map[string]int64),
		},
	}

	// Initialize rate limiter if rate limiting is enabled
	if agent.rateLimit > 0 {
		agent.rateLimiter = make(chan struct{}, agent.rateLimit)
		go agent.startRateLimiter()
	}

	agent.setupHTTPClient()

	LogInfo("Attempting to connect to coordinator at %s:%d...", masterHost, masterPort)

	if err := agent.connectToCoordinator(); err != nil {
		LogError("Failed to connect to coordinator: %v", err)
		LogInfo("Make sure the coordinator is running:")
		LogInfo("  ./armonite coordinator --plan <test-plan.yaml>")
		return fmt.Errorf("coordinator not available at %s:%d", masterHost, masterPort)
	}
	defer agent.natsConn.Close()

	// Register with coordinator
	if err := agent.registerWithCoordinator(); err != nil {
		LogWarn("Failed to register with coordinator: %v", err)
	}

	agent.subscribeToTestCommands()
	agent.startMetricsReporting()
	agent.startHeartbeat()
	agent.startProgressDisplay()

	LogInfo("Agent %s started, connecting to %s:%d", id, masterHost, masterPort)
	LogInfo("Concurrency: %d, Keep-Alive: %t, Region: %s", concurrency, keepAlive, region)

	if agent.devMode {
		LogInfo("Development mode: Rate limit: %d req/s, Default think time: %s",
			agent.rateLimit, agent.defaultThinkTime)
	}

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	LogInfo("Shutting down agent...")
	agent.unregisterFromCoordinator()
	agent.stop()
	return nil
}

func (a *Agent) setupHTTPClient() {
	transport := &http.Transport{
		DisableKeepAlives: !a.keepAlive,
		MaxIdleConns:      a.concurrency,
		IdleConnTimeout:   30 * time.Second,
	}

	a.httpClient = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

func (a *Agent) connectToCoordinator() error {
	// Try to connect to NATS with timeout and retries
	natsURL := fmt.Sprintf("nats://%s:%d", a.masterHost, a.masterPort)

	// Set connection options with timeout
	opts := []nats.Option{
		nats.Timeout(5 * time.Second),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(3),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			LogWarn("Disconnected from coordinator: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			LogInfo("Reconnected to coordinator")
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			LogInfo("Connection to coordinator closed")
		}),
	}

	var err error
	a.natsConn, err = nats.Connect(natsURL, opts...)
	if err != nil {
		return fmt.Errorf("coordinator unavailable: %w", err)
	}

	// Test connection with a simple publish
	if err := a.natsConn.Publish("armonite.ping", []byte("ping")); err != nil {
		return fmt.Errorf("coordinator not responding: %w", err)
	}

	LogInfo("Successfully connected to coordinator")
	return nil
}

func (a *Agent) subscribeToTestCommands() {
	// Global test commands (for backward compatibility and non-phase tests)
	a.natsConn.Subscribe("armonite.test.command", func(msg *nats.Msg) {
		a.handleTestCommand(msg)
	})

	// Agent-specific commands for phase coordination
	agentSpecificSubject := fmt.Sprintf("armonite.agent.%s.command", a.id)
	a.natsConn.Subscribe(agentSpecificSubject, func(msg *nats.Msg) {
		a.handleAgentCommand(msg)
	})
}

func (a *Agent) handleTestCommand(msg *nats.Msg) {
	var command TestStartCommand
	if err := json.Unmarshal(msg.Data, &command); err != nil {
		LogError("Failed to unmarshal test command: %v", err)
		return
	}

	switch command.Command {
	case "START":
		if command.TestRunID != "" {
			LogInfo("Received test plan: %s (Test Run ID: %s)", command.TestPlan.Name, command.TestRunID)
			a.currentTestRunID = command.TestRunID
		} else {
			LogInfo("Received test plan: %s", command.TestPlan.Name)
		}
		LogInfo("Test configuration - Duration: %s, Concurrency: %d, Endpoints: %d",
			command.TestPlan.Duration, a.concurrency, len(command.TestPlan.Endpoints))
		LogInfo("Starting test execution...")
		a.sendExecutionUpdate("starting", fmt.Sprintf("Starting test execution: %s", command.TestPlan.Name))
		a.executeTestPlan(&command.TestPlan)
	case "STOP":
		if command.TestRunID != "" && command.TestRunID != a.currentTestRunID {
			LogDebug("Ignoring stop command for different test run: %s (current: %s)", command.TestRunID, a.currentTestRunID)
			return
		}
		LogInfo("Received test stop command")
		a.sendExecutionUpdate("stopping", "Received stop command from coordinator")
		a.stopTest()
	}
}

func (a *Agent) handleAgentCommand(msg *nats.Msg) {
	var command TestStartCommand
	if err := json.Unmarshal(msg.Data, &command); err != nil {
		LogError("Failed to unmarshal agent command: %v", err)
		return
	}

	switch command.Command {
	case "START_PHASE":
		a.executePhase(command.CurrentPhase)
	case "STOP_PHASE":
		a.stopCurrentPhase()
	}
}

func (a *Agent) executeTestPlan(plan *TestPlan) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return // Already running a test
	}
	a.running = true
	a.testStarted = true
	a.testCompleted = false
	a.currentPlan = plan
	a.resetMetrics()

	// Initialize ramp-up strategy
	var rampUpStrategy RampUpStrategy
	if plan.RampUpStrategy != nil {
		rampUpStrategy = *plan.RampUpStrategy
	} else {
		// Use default immediate ramp-up if no strategy specified
		rampUpStrategy = CreateDefaultRampUp()
	}

	// Create and initialize ramp-up calculator
	var err error
	a.rampUpCalculator, err = NewRampUpCalculator(rampUpStrategy, a.concurrency)
	if err != nil {
		LogWarn("Failed to create ramp-up calculator: %v, using immediate ramp-up", err)
		a.rampUpCalculator, _ = NewRampUpCalculator(CreateDefaultRampUp(), a.concurrency)
	}

	// Start ramp-up execution
	a.rampUpExecution = a.rampUpCalculator.Start()
	a.mu.Unlock()

	duration, err := time.ParseDuration(plan.Duration)
	if err != nil {
		LogWarn("Invalid duration %s, using 1m", plan.Duration)
		duration = time.Minute
	}

	LogInfo("Starting load test: %s for %s with ramp-up strategy: %s",
		plan.Name, duration, rampUpStrategy.Type)

	a.sendExecutionUpdate("running", fmt.Sprintf("Load test running with %s ramp-up, %s duration",
		rampUpStrategy.Type, duration))

	// Start workers with dynamic concurrency based on ramp-up strategy
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	requestCh := make(chan Endpoint, a.concurrency*10) // Buffered channel for requests
	workerControlCh := make(chan int, 1)               // Channel to control number of active workers

	// Start request generator
	go a.generateRequests(plan.Endpoints, requestCh, stopCh)

	// Start ramp-up controller goroutine
	go a.rampUpController(workerControlCh, stopCh, &wg)

	// Start with initial number of workers
	initialConcurrency := a.rampUpCalculator.GetCurrentConcurrency(a.rampUpExecution)
	a.startWorkers(initialConcurrency, requestCh, stopCh, &wg)
	workerControlCh <- initialConcurrency

	// Stop after duration
	time.AfterFunc(duration, func() {
		close(stopCh)
	})

	wg.Wait()

	a.mu.Lock()
	a.running = false
	if !a.testCompleted {
		a.testCompleted = true
	}

	// Get final metrics for reporting
	a.metrics.mu.Lock()
	requests := a.metrics.Requests
	errors := a.metrics.Errors
	a.metrics.mu.Unlock()
	a.mu.Unlock()

	LogInfo("Load test completed")
	a.sendExecutionUpdate("completed", fmt.Sprintf("Test completed: %d requests, %d errors", requests, errors))
}

func (a *Agent) generateRequests(endpoints []Endpoint, requestCh chan<- Endpoint, stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			close(requestCh)
			return
		default:
			for _, endpoint := range endpoints {
				select {
				case <-stopCh:
					close(requestCh)
					return
				case requestCh <- endpoint:
					// Request queued successfully
				}
			}
		}
	}
}

// rampUpController manages dynamic worker scaling based on ramp-up strategy
func (a *Agent) rampUpController(workerControlCh <-chan int, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	ticker := time.NewTicker(1 * time.Second) // Check ramp-up status every second
	defer ticker.Stop()

	currentWorkers := 0

	for {
		select {
		case <-stopCh:
			return
		case workers := <-workerControlCh:
			currentWorkers = workers
		case <-ticker.C:
			a.mu.RLock()
			if a.rampUpCalculator != nil && a.rampUpExecution != nil {
				targetConcurrency := a.rampUpCalculator.GetCurrentConcurrency(a.rampUpExecution)

				if targetConcurrency != currentWorkers {
					LogDebug("Ramp-up adjustment: %d -> %d workers", currentWorkers, targetConcurrency)
					// Note: For simplicity, we're not dynamically adding/removing workers during execution
					// Instead, we log the target concurrency change for monitoring
					currentWorkers = targetConcurrency
				}

				// Check if ramp-up is complete
				if a.rampUpCalculator.IsComplete(a.rampUpExecution) {
					remaining := a.rampUpCalculator.GetRemainingDuration(a.rampUpExecution)
					if remaining <= 0 {
						LogInfo("Ramp-up phase completed, running at full concurrency: %d", a.concurrency)
					}
				}
			}
			a.mu.RUnlock()
		}
	}
}

// startWorkers starts the specified number of worker goroutines
func (a *Agent) startWorkers(count int, requestCh <-chan Endpoint, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			a.worker(workerID, requestCh, stopCh)
		}(i)
	}
}

func (a *Agent) worker(workerID int, requestCh <-chan Endpoint, stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		case endpoint, ok := <-requestCh:
			if !ok {
				return // Channel closed
			}

			// Check if we should throttle based on current ramp-up state
			a.mu.RLock()
			shouldThrottle := false
			if a.rampUpCalculator != nil && a.rampUpExecution != nil {
				targetConcurrency := a.rampUpCalculator.GetCurrentConcurrency(a.rampUpExecution)
				// Simple throttling: if we're above target concurrency, add delay
				if workerID >= targetConcurrency {
					shouldThrottle = true
				}
			}
			a.mu.RUnlock()

			if shouldThrottle {
				// Throttle this worker by adding a small delay
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Apply rate limiting
			a.waitForRateLimit()

			// Execute each HTTP request in its own goroutine
			go a.executeRequest(endpoint)

			// Apply think time (endpoint-specific or default)
			thinkTime := a.getEffectiveThinkTime(endpoint)
			if thinkTime > 0 {
				time.Sleep(thinkTime)
			}
		}
	}
}

func (a *Agent) executeRequest(endpoint Endpoint) {
	start := time.Now()

	var body io.Reader
	if endpoint.Body != nil {
		bodyData, _ := json.Marshal(endpoint.Body)
		body = bytes.NewReader(bodyData)
	}

	req, err := http.NewRequest(endpoint.Method, endpoint.URL, body)
	if err != nil {
		a.recordError()
		return
	}

	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.recordError()
		return
	}
	defer resp.Body.Close()

	// Read response body to ensure connection is properly closed
	io.Copy(io.Discard, resp.Body)

	latency := time.Since(start)
	a.recordRequest(resp.StatusCode, latency)
}

func (a *Agent) recordRequest(statusCode int, latency time.Duration) {
	a.metrics.mu.Lock()
	defer a.metrics.mu.Unlock()

	latencyMs := float64(latency.Milliseconds())

	a.metrics.Requests++
	a.metrics.totalLatency += latencyMs
	a.metrics.AvgLatencyMs = a.metrics.totalLatency / float64(a.metrics.Requests)

	// Update min/max latency
	if a.metrics.Requests == 1 {
		// First request - initialize min/max
		a.metrics.MinLatencyMs = latencyMs
		a.metrics.MaxLatencyMs = latencyMs
	} else {
		if latencyMs < a.metrics.MinLatencyMs {
			a.metrics.MinLatencyMs = latencyMs
		}
		if latencyMs > a.metrics.MaxLatencyMs {
			a.metrics.MaxLatencyMs = latencyMs
		}
	}

	statusStr := fmt.Sprintf("%d", statusCode)
	a.metrics.StatusCodes[statusStr]++
}

func (a *Agent) recordError() {
	a.metrics.mu.Lock()
	defer a.metrics.mu.Unlock()

	a.metrics.Errors++
}

func (a *Agent) resetMetrics() {
	a.metrics.mu.Lock()
	defer a.metrics.mu.Unlock()

	a.metrics.Requests = 0
	a.metrics.Errors = 0
	a.metrics.AvgLatencyMs = 0
	a.metrics.MinLatencyMs = 0
	a.metrics.MaxLatencyMs = 0
	a.metrics.totalLatency = 0
	a.metrics.StatusCodes = make(map[string]int64)
}

func (a *Agent) startMetricsReporting() {
	// Start with more frequent reporting, then reduce frequency
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		reportCount := 0
		for range ticker.C {
			a.reportMetrics()
			reportCount++

			// After 3 reports (6 seconds), switch to less frequent reporting
			if reportCount == 3 {
				ticker.Stop()
				ticker = time.NewTicker(5 * time.Second)
				go func() {
					for range ticker.C {
						a.reportMetrics()
					}
				}()
				return
			}
		}
	}()
}

func (a *Agent) reportMetrics() {
	a.metrics.mu.Lock()
	if a.metrics.Requests == 0 {
		a.metrics.mu.Unlock()
		return
	}

	a.metrics.Timestamp = time.Now().UTC().Format(time.RFC3339)
	metricsData, err := json.Marshal(a.metrics)
	a.metrics.mu.Unlock()

	if err != nil {
		LogError("Failed to marshal metrics: %v", err)
		return
	}

	if err := a.natsConn.Publish("armonite.telemetry", metricsData); err != nil {
		LogError("Failed to publish metrics: %v", err)
	}
}

func (a *Agent) executePhase(phase *PhaseInfo) {
	if phase == nil {
		LogError("Received nil phase info")
		return
	}

	a.mu.Lock()
	a.currentPhase = phase
	a.phaseStopCh = make(chan struct{})
	a.mu.Unlock()

	LogInfo("Agent %s executing phase %d in %s mode (concurrency: %d, duration: %s)",
		a.id, phase.PhaseIndex, phase.Mode, phase.Concurrency, phase.Duration)

	// Set phase-specific concurrency
	phaseConcurrency := phase.Concurrency
	if phaseConcurrency > a.concurrency {
		phaseConcurrency = a.concurrency // Don't exceed agent's max capacity
	}

	duration, err := time.ParseDuration(phase.Duration)
	if err != nil {
		LogWarn("Invalid phase duration %s, using 30s", phase.Duration)
		duration = 30 * time.Second
	}

	// Execute load for this phase
	a.executePhaseLoad(phaseConcurrency, duration, phase)

	// Send completion signal
	completion := PhaseCompletion{
		AgentID:     a.id,
		PhaseID:     phase.PhaseID,
		PhaseIndex:  phase.PhaseIndex,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(completion)
	if err != nil {
		LogError("Failed to marshal phase completion: %v", err)
		return
	}

	// Send completion to coordinator
	subject := fmt.Sprintf("armonite.phase.complete.%s", a.currentTestRunID)
	if err := a.natsConn.Publish(subject, data); err != nil {
		LogError("Failed to send phase completion: %v", err)
	}

	LogInfo("Agent %s completed phase %d", a.id, phase.PhaseIndex)
}

func (a *Agent) executePhaseLoad(concurrency int, duration time.Duration, phase *PhaseInfo) {
	a.sendExecutionUpdate("running", fmt.Sprintf("Phase %d: %s mode, %d workers, %s duration",
		phase.PhaseIndex, phase.Mode, concurrency, duration))

	// Start workers for this phase
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	requestCh := make(chan Endpoint, concurrency*10)

	// Start worker goroutines
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			a.phaseWorker(workerID, requestCh, stopCh)
		}(i)
	}

	// Start request generator if we have a current plan
	a.mu.RLock()
	if a.currentPlan != nil {
		go a.generateRequests(a.currentPlan.Endpoints, requestCh, stopCh)
	}
	a.mu.RUnlock()

	// Stop after duration or when phase is cancelled
	phaseTimer := time.NewTimer(duration)
	defer phaseTimer.Stop()

	select {
	case <-phaseTimer.C:
		LogDebug("Phase %d completed by timeout", phase.PhaseIndex)
	case <-a.phaseStopCh:
		LogDebug("Phase %d stopped by coordinator", phase.PhaseIndex)
	}

	close(stopCh)
	wg.Wait()
}

func (a *Agent) phaseWorker(workerID int, requestCh <-chan Endpoint, stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		case endpoint, ok := <-requestCh:
			if !ok {
				return // Channel closed
			}

			// Apply rate limiting
			a.waitForRateLimit()

			// Execute HTTP request
			go a.executeRequest(endpoint)

			// Apply think time (endpoint-specific or default)
			thinkTime := a.getEffectiveThinkTime(endpoint)
			if thinkTime > 0 {
				time.Sleep(thinkTime)
			}
		}
	}
}

func (a *Agent) stopCurrentPhase() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.phaseStopCh != nil {
		close(a.phaseStopCh)
		a.phaseStopCh = nil
	}

	LogInfo("Agent %s stopped current phase", a.id)
}

func (a *Agent) sendExecutionUpdate(status, message string) {
	update := map[string]interface{}{
		"agent_id":  a.id,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"status":    status,
		"message":   message,
	}

	if a.currentTestRunID != "" {
		update["test_run_id"] = a.currentTestRunID
	}

	data, err := json.Marshal(update)
	if err != nil {
		LogError("Failed to marshal execution update: %v", err)
		return
	}

	if err := a.natsConn.Publish("armonite.agent.status", data); err != nil {
		LogError("Failed to send execution update: %v", err)
	}
}

func (a *Agent) stopTest() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.running = false
	a.testCompleted = true

	// Stop any running phase
	if a.phaseStopCh != nil {
		close(a.phaseStopCh)
		a.phaseStopCh = nil
	}

	LogInfo("Agent %s stopped test execution", a.id)
}

// startRateLimiter fills the rate limiter channel at the specified rate
func (a *Agent) startRateLimiter() {
	if a.rateLimit <= 0 {
		return
	}

	ticker := time.NewTicker(time.Second / time.Duration(a.rateLimit))
	defer ticker.Stop()

	for range ticker.C {
		select {
		case a.rateLimiter <- struct{}{}:
		default:
			// Channel is full, skip this tick
		}
	}
}

// waitForRateLimit blocks until rate limit allows the request
func (a *Agent) waitForRateLimit() {
	if a.rateLimiter != nil {
		<-a.rateLimiter
	}
}

// getEffectiveThinkTime returns the think time to use, preferring endpoint-specific over default
func (a *Agent) getEffectiveThinkTime(endpoint Endpoint) time.Duration {
	// Use endpoint-specific think time if available
	if endpoint.ThinkTime != "" {
		if thinkTime, err := time.ParseDuration(endpoint.ThinkTime); err == nil {
			return thinkTime
		}
	}

	// Fall back to agent default think time
	return a.defaultThinkTime
}

func (a *Agent) stop() {
	a.mu.Lock()
	a.running = false

	// Stop any running phase
	if a.phaseStopCh != nil {
		close(a.phaseStopCh)
		a.phaseStopCh = nil
	}

	a.mu.Unlock()
}
