package main

import (
	"encoding/json"
	"time"
)

func (c *Coordinator) startStatusDisplay() {
	// This function can be kept simple since we have HTTP endpoints
	LogDebug("Status display functionality available via HTTP API")
}

func (c *Coordinator) broadcastTestStart(testRun *TestRun) error {
	c.mu.Lock()
	agentCount := len(c.connectedAgents)

	// Initialize ramp-up execution for all agents if strategy is defined
	if testRun.TestPlan.RampUpStrategy != nil {
		for _, agent := range c.connectedAgents {
			if agent.RampUpExecution == nil {
				calculator, calcErr := NewRampUpCalculator(*testRun.TestPlan.RampUpStrategy, agent.Concurrency)
				if calcErr == nil {
					agent.RampUpExecution = calculator.Start()
					LogDebug("Initialized ramp-up execution for agent %s with strategy %s",
						agent.ID, testRun.TestPlan.RampUpStrategy.Type)
				}
			}
		}
	}

	// Check if we need phase orchestration for custom strategies with specific modes
	needsPhaseOrchestration := c.hasSequentialPhases(testRun.TestPlan.RampUpStrategy)

	if needsPhaseOrchestration {
		// Start phase orchestration instead of simple broadcast
		c.phaseOrchestrator = NewPhaseOrchestrator(testRun.ID, &testRun.TestPlan, c.natsConn, c.connectedAgents)
		c.mu.Unlock()

		LogInfo("Starting phase orchestration for test run: %s", testRun.Name)
		c.phaseOrchestrator.Start()

		return nil
	}
	c.mu.Unlock()

	// Use standard broadcast for non-sequential tests
	testStart := TestStartCommand{
		TestRunID: testRun.ID,
		TestPlan:  testRun.TestPlan,
		StartTime: time.Now().UTC().Format(time.RFC3339),
		Command:   "START",
	}

	data, err := json.Marshal(testStart)
	if err != nil {
		return err
	}

	if err := c.natsConn.Publish("armonite.test.command", data); err != nil {
		return err
	}

	LogInfo("Test start command sent to %d agents for test run: %s", agentCount, testRun.Name)
	if testRun.TestPlan.RampUpStrategy != nil {
		LogInfo("Ramp-up strategy enabled: %s (duration: %s)",
			testRun.TestPlan.RampUpStrategy.Type, testRun.TestPlan.RampUpStrategy.Duration)
	}
	return nil
}

// hasSequentialPhases checks if the ramp-up strategy contains any sequential phases
func (c *Coordinator) hasSequentialPhases(strategy *RampUpStrategy) bool {
	if strategy == nil || strategy.Type != "custom" {
		return false
	}

	for _, phase := range strategy.Phases {
		if phase.Mode == "sequential" {
			return true
		}
	}

	return false
}

func (c *Coordinator) completeTestRun(testRunID string) {
	c.mu.Lock()

	testRun, exists := c.testRuns[testRunID]
	if !exists {
		c.mu.Unlock()
		return
	}

	// Stop phase orchestration if it's running
	if c.phaseOrchestrator != nil {
		c.phaseOrchestrator.Stop()
		c.phaseOrchestrator = nil
	}

	c.mu.Unlock()

	// Collect results from agent data
	agentResults := c.agentResults[testRunID]
	if agentResults == nil {
		agentResults = []AgentResult{}
	}

	// Calculate aggregate results
	var totalRequests, totalErrors int64
	var totalLatency float64
	var minLatency, maxLatency float64
	var requestsPerSec float64
	statusCodes := make(map[string]int64)

	// Initialize min/max latency
	if len(agentResults) > 0 {
		minLatency = agentResults[0].MinLatencyMs
		maxLatency = agentResults[0].MaxLatencyMs
	}

	for _, result := range agentResults {
		totalRequests += result.Requests
		totalErrors += result.Errors
		totalLatency += result.AvgLatencyMs * float64(result.Requests)

		// Track global min/max latency across all agents
		if result.MinLatencyMs > 0 && (minLatency == 0 || result.MinLatencyMs < minLatency) {
			minLatency = result.MinLatencyMs
		}
		if result.MaxLatencyMs > maxLatency {
			maxLatency = result.MaxLatencyMs
		}

		for code, count := range result.StatusCodes {
			statusCodes[code] += count
		}
	}

	var avgLatency float64
	if totalRequests > 0 {
		avgLatency = totalLatency / float64(totalRequests)
	}

	successRate := float64(100)
	if totalRequests > 0 {
		successRate = float64(totalRequests-totalErrors) / float64(totalRequests) * 100
	}

	// Calculate requests per second based on test duration
	if testRun.StartedAt != nil && testRun.CompletedAt != nil {
		durationSeconds := testRun.CompletedAt.Sub(*testRun.StartedAt).Seconds()
		if durationSeconds > 0 {
			requestsPerSec = float64(totalRequests) / durationSeconds
		}
	}

	results := &TestRunResults{
		TotalRequests:  totalRequests,
		TotalErrors:    totalErrors,
		SuccessRate:    successRate,
		AvgLatencyMs:   avgLatency,
		MinLatencyMs:   minLatency,
		MaxLatencyMs:   maxLatency,
		RequestsPerSec: requestsPerSec,
		StatusCodes:    statusCodes,
		AgentResults:   agentResults,
	}

	testRun.Complete(results)

	// Save to database
	if err := c.database.SaveTestRun(testRun); err != nil {
		LogError("Failed to save completed test run to database: %v", err)
	}

	// Clear current test run if this is it
	if c.currentTestRun != nil && c.currentTestRun.ID == testRunID {
		c.currentTestRun = nil
	}

	LogInfo("Test run completed: %s", testRun.Name)
}

type TestStartCommand struct {
	TestRunID    string     `json:"test_run_id,omitempty"`
	TestPlan     TestPlan   `json:"test_plan,omitempty"`
	StartTime    string     `json:"start_time,omitempty"`
	Command      string     `json:"command"` // START, STOP, START_PHASE, STOP_PHASE
	CurrentPhase *PhaseInfo `json:"current_phase,omitempty"`
}

type PhaseInfo struct {
	PhaseIndex  int      `json:"phase_index"`
	PhaseID     string   `json:"phase_id"`
	Mode        string   `json:"mode"`
	Concurrency int      `json:"concurrency"`
	Duration    string   `json:"duration"`
	AgentOrder  []string `json:"agent_order,omitempty"` // For sequential mode
}

type PhaseCompletion struct {
	AgentID     string `json:"agent_id"`
	PhaseID     string `json:"phase_id"`
	PhaseIndex  int    `json:"phase_index"`
	CompletedAt string `json:"completed_at"`
}
