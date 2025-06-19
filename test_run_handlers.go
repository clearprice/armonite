package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (c *Coordinator) handleCreateTestRun(ctx *gin.Context) {
	var req CreateTestRunRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	// Validate test plan
	if req.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Test run name is required"})
		return
	}

	if len(req.TestPlan.Endpoints) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Test plan must have at least one endpoint"})
		return
	}

	// Set default min agents if not specified
	if req.MinAgents == 0 {
		req.MinAgents = c.config.Defaults.MinAgents
	}

	// Create test run
	testRun := NewTestRun(req.Name, req.TestPlan, req.MinAgents, req.Parameters)

	c.mu.Lock()
	c.testRuns[testRun.ID] = testRun
	c.mu.Unlock()

	// Save to database
	if err := c.database.SaveTestRun(testRun); err != nil {
		LogError("Failed to save test run to database: %v", err)
		// Continue anyway - in-memory storage still works
	}

	LogInfo("Test run created: %s (ID: %s)", testRun.Name, testRun.ID)

	ctx.JSON(http.StatusCreated, testRun)
}

func (c *Coordinator) handleListTestRuns(ctx *gin.Context) {
	// Try to get from database first, fall back to in-memory
	testRuns, err := c.database.ListTestRuns(100, 0)
	if err != nil {
		LogWarn("Failed to load test runs from database, using in-memory: %v", err)

		c.mu.RLock()
		testRuns = make([]*TestRun, 0, len(c.testRuns))
		for _, tr := range c.testRuns {
			testRuns = append(testRuns, tr)
		}
		c.mu.RUnlock()
	}

	ctx.JSON(http.StatusOK, gin.H{
		"test_runs": testRuns,
		"total":     len(testRuns),
	})
}

func (c *Coordinator) handleGetTestRun(ctx *gin.Context) {
	testRunID := ctx.Param("id")

	// Try database first, then in-memory
	testRun, err := c.database.GetTestRun(testRunID)
	if err != nil {
		c.mu.RLock()
		testRun, exists := c.testRuns[testRunID]
		c.mu.RUnlock()

		if !exists {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
			return
		}

		ctx.JSON(http.StatusOK, testRun)
		return
	}

	ctx.JSON(http.StatusOK, testRun)
}

func (c *Coordinator) handleGetTestRunResults(ctx *gin.Context) {
	testRunID := ctx.Param("id")

	// Get test run from database (authoritative source)
	testRun, err := c.database.GetTestRun(testRunID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	// Get agent results from database
	agentResults, err := c.database.GetAgentResults(testRunID)
	if err != nil {
		LogWarn("Failed to get agent results from database for test run %s: %v", testRunID, err)
		agentResults = []AgentResult{}
	}

	// Return detailed results including agent-level data
	results := gin.H{
		"test_run":      testRun,
		"summary":       testRun.Results,
		"agent_results": agentResults,
	}

	ctx.JSON(http.StatusOK, results)
}

func (c *Coordinator) handleStartTestRun(ctx *gin.Context) {
	testRunID := ctx.Param("id")

	c.mu.Lock()
	defer c.mu.Unlock()

	testRun, exists := c.testRuns[testRunID]
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	// Check if test can be started
	if testRun.Status != TestRunStatusCreated {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":  "Test run cannot be started",
			"status": testRun.Status,
		})
		return
	}

	// Check if another test is already running
	if c.currentTestRun != nil && (c.currentTestRun.Status == TestRunStatusRunning || c.currentTestRun.Status == TestRunStatusWaiting) {
		ctx.JSON(http.StatusConflict, gin.H{
			"error":           "Another test run is already active",
			"active_test_run": c.currentTestRun.ID,
		})
		return
	}

	// Start the test run
	testRun.Start()
	c.currentTestRun = testRun

	// Save to database
	if err := c.database.SaveTestRun(testRun); err != nil {
		LogError("Failed to save started test run to database: %v", err)
	}

	// Check if we have enough agents to start immediately
	if len(c.connectedAgents) >= testRun.AgentCount {
		LogInfo("Starting test run immediately: %s (%d agents available)", testRun.Name, len(c.connectedAgents))
		go c.startTestRun(testRun)
	} else {
		testRun.Status = TestRunStatusWaiting
		LogInfo("Test run waiting for agents: %s (%d/%d agents)", testRun.Name, len(c.connectedAgents), testRun.AgentCount)

		// Save status update to database
		if err := c.database.SaveTestRun(testRun); err != nil {
			LogError("Failed to save waiting test run to database: %v", err)
		}
	}

	LogInfo("Test run started: %s (ID: %s)", testRun.Name, testRun.ID)

	ctx.JSON(http.StatusOK, testRun)
}

func (c *Coordinator) handleStopTestRun(ctx *gin.Context) {
	testRunID := ctx.Param("id")

	c.mu.Lock()
	defer c.mu.Unlock()

	testRun, exists := c.testRuns[testRunID]
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	if testRun.Status != TestRunStatusRunning && testRun.Status != TestRunStatusWaiting {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":  "Test run is not running",
			"status": testRun.Status,
		})
		return
	}

	// Send stop command to agents
	go c.stopTestRun(testRun)

	LogInfo("Test run stop requested: %s (ID: %s)", testRun.Name, testRun.ID)

	ctx.JSON(http.StatusOK, gin.H{"message": "Test run stop initiated"})
}

func (c *Coordinator) handleDeleteTestRun(ctx *gin.Context) {
	testRunID := ctx.Param("id")

	c.mu.Lock()
	defer c.mu.Unlock()

	testRun, exists := c.testRuns[testRunID]
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	// Only allow deletion of completed, failed, or cancelled test runs
	if testRun.Status == TestRunStatusRunning || testRun.Status == TestRunStatusWaiting {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":  "Cannot delete active test run",
			"status": testRun.Status,
		})
		return
	}

	// Clear current test run if this is it
	if c.currentTestRun != nil && c.currentTestRun.ID == testRunID {
		c.currentTestRun = nil
	}

	// Remove from database
	if err := c.database.DeleteTestRun(testRunID); err != nil {
		LogError("Failed to delete test run from database: %v", err)
	}

	// Remove from memory
	delete(c.testRuns, testRunID)
	delete(c.agentResults, testRunID)

	LogInfo("Test run deleted: %s (ID: %s)", testRun.Name, testRun.ID)

	ctx.JSON(http.StatusOK, gin.H{"message": "Test run deleted"})
}

type BulkDeleteRequest struct {
	Status    string `json:"status,omitempty"`     // Delete by status (e.g., "completed", "failed")
	OlderThan string `json:"older_than,omitempty"` // Delete older than duration (e.g., "7d", "30d")
	Confirm   bool   `json:"confirm"`              // Safety confirmation
}

func (c *Coordinator) handleBulkDeleteTestRuns(ctx *gin.Context) {
	var req BulkDeleteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	// Safety check - require explicit confirmation
	if !req.Confirm {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Bulk deletion requires confirmation (set confirm: true)"})
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var deletedCount int64
	var err error

	if req.Status != "" {
		// Delete by status
		deletedCount, err = c.database.DeleteTestRunsByStatus(req.Status)
		if err != nil {
			LogError("Failed to bulk delete test runs by status %s: %v", req.Status, err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete test runs", "details": err.Error()})
			return
		}

		// Remove from memory
		for id, testRun := range c.testRuns {
			if string(testRun.Status) == req.Status {
				// Don't delete active test runs from memory
				if testRun.Status != TestRunStatusRunning && testRun.Status != TestRunStatusWaiting {
					delete(c.testRuns, id)
					delete(c.agentResults, id)
				}
			}
		}

		LogInfo("Bulk deleted %d test runs with status: %s", deletedCount, req.Status)
		ctx.JSON(http.StatusOK, gin.H{
			"message":       fmt.Sprintf("Deleted %d test runs with status: %s", deletedCount, req.Status),
			"deleted_count": deletedCount,
		})

	} else if req.OlderThan != "" {
		// Delete by age
		duration, parseErr := time.ParseDuration(req.OlderThan)
		if parseErr != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid duration format", "details": parseErr.Error()})
			return
		}

		deletedCount, err = c.database.DeleteTestRunsOlderThan(duration)
		if err != nil {
			LogError("Failed to bulk delete test runs older than %s: %v", req.OlderThan, err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete test runs", "details": err.Error()})
			return
		}

		// Remove from memory
		cutoffTime := time.Now().Add(-duration)
		for id, testRun := range c.testRuns {
			if testRun.CreatedAt.Before(cutoffTime) {
				// Don't delete active test runs from memory
				if testRun.Status != TestRunStatusRunning && testRun.Status != TestRunStatusWaiting {
					delete(c.testRuns, id)
					delete(c.agentResults, id)
				}
			}
		}

		LogInfo("Bulk deleted %d test runs older than: %s", deletedCount, req.OlderThan)
		ctx.JSON(http.StatusOK, gin.H{
			"message":       fmt.Sprintf("Deleted %d test runs older than %s", deletedCount, req.OlderThan),
			"deleted_count": deletedCount,
		})

	} else {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Must specify either 'status' or 'older_than' parameter"})
	}
}

func (c *Coordinator) handleTestRunStats(ctx *gin.Context) {
	// Get stats from database
	totalCount, err := c.database.GetTestRunCount()
	if err != nil {
		LogError("Failed to get test run count: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve statistics"})
		return
	}

	// Get counts by status
	statusCounts := make(map[string]int64)
	statuses := []string{"created", "waiting", "running", "completed", "failed", "cancelled"}

	for _, status := range statuses {
		count, err := c.database.GetTestRunCountByStatus(status)
		if err != nil {
			LogWarn("Failed to get count for status %s: %v", status, err)
			count = 0
		}
		statusCounts[status] = count
	}

	// Get in-memory stats
	c.mu.RLock()
	inMemoryCount := len(c.testRuns)
	agentResultsCount := len(c.agentResults)
	var currentTestRun *TestRun
	if c.currentTestRun != nil {
		currentTestRun = c.currentTestRun
	}
	c.mu.RUnlock()

	stats := gin.H{
		"total_test_runs":  totalCount,
		"status_breakdown": statusCounts,
		"in_memory": gin.H{
			"test_runs":     inMemoryCount,
			"agent_results": agentResultsCount,
		},
		"current_test_run": nil,
	}

	if currentTestRun != nil {
		stats["current_test_run"] = gin.H{
			"id":     currentTestRun.ID,
			"name":   currentTestRun.Name,
			"status": currentTestRun.Status,
		}
	}

	ctx.JSON(http.StatusOK, stats)
}

func (c *Coordinator) handleRerunTestRun(ctx *gin.Context) {
	testRunID := ctx.Param("id")

	c.mu.Lock()
	defer c.mu.Unlock()

	originalTestRun, exists := c.testRuns[testRunID]
	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Test run not found"})
		return
	}

	// Only allow rerun of completed, failed, or cancelled test runs
	if originalTestRun.Status == TestRunStatusRunning || originalTestRun.Status == TestRunStatusWaiting || originalTestRun.Status == TestRunStatusCreated {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":  "Can only rerun completed, failed, or cancelled test runs",
			"status": originalTestRun.Status,
		})
		return
	}

	// Check if another test is already running
	if c.currentTestRun != nil && (c.currentTestRun.Status == TestRunStatusRunning || c.currentTestRun.Status == TestRunStatusWaiting) {
		ctx.JSON(http.StatusConflict, gin.H{
			"error":           "Another test run is already active",
			"active_test_run": c.currentTestRun.ID,
		})
		return
	}

	// Create a new test run based on the original
	newTestRun := NewTestRun(
		originalTestRun.Name+" (Rerun)",
		originalTestRun.TestPlan,
		originalTestRun.AgentCount,
		originalTestRun.Parameters,
	)

	// Store the new test run
	c.testRuns[newTestRun.ID] = newTestRun

	// Save to database
	if err := c.database.SaveTestRun(newTestRun); err != nil {
		LogError("Failed to save rerun test run to database: %v", err)
	}

	// Start the new test run immediately
	newTestRun.Start()
	c.currentTestRun = newTestRun

	// Save start status to database
	if err := c.database.SaveTestRun(newTestRun); err != nil {
		LogError("Failed to save started rerun test run to database: %v", err)
	}

	// Check if we have enough agents to start immediately
	if len(c.connectedAgents) >= newTestRun.AgentCount {
		LogInfo("Starting rerun test immediately: %s (%d agents available)", newTestRun.Name, len(c.connectedAgents))
		go c.startTestRun(newTestRun)
	} else {
		newTestRun.Status = TestRunStatusWaiting
		LogInfo("Rerun test waiting for agents: %s (%d/%d agents)", newTestRun.Name, len(c.connectedAgents), newTestRun.AgentCount)

		// Save waiting status to database
		if err := c.database.SaveTestRun(newTestRun); err != nil {
			LogError("Failed to save waiting rerun test run to database: %v", err)
		}
	}

	LogInfo("Test run rerun started: %s (Original ID: %s, New ID: %s)", newTestRun.Name, testRunID, newTestRun.ID)

	ctx.JSON(http.StatusCreated, newTestRun)
}

type TestConnectionRequest struct {
	URL     string                 `json:"url"`
	Method  string                 `json:"method"`
	Headers map[string]string      `json:"headers"`
	Body    map[string]interface{} `json:"body,omitempty"`
}

type TestConnectionResponse struct {
	Success      bool              `json:"success"`
	StatusCode   int               `json:"status_code,omitempty"`
	ResponseTime float64           `json:"response_time_ms,omitempty"`
	Error        string            `json:"error,omitempty"`
	Headers      map[string]string `json:"response_headers,omitempty"`
	BodyPreview  string            `json:"body_preview,omitempty"`
}

func (c *Coordinator) handleTestConnection(ctx *gin.Context) {
	var req TestConnectionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	// Validate required fields
	if req.URL == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	if req.Method == "" {
		req.Method = "GET"
	}

	// Test the connection
	result := c.testEndpointConnection(req)
	ctx.JSON(http.StatusOK, result)
}

func (c *Coordinator) testEndpointConnection(req TestConnectionRequest) TestConnectionResponse {
	start := time.Now()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Allow self-signed certificates for testing
		},
	}

	// Prepare request body
	var bodyReader io.Reader
	if req.Method != "GET" && len(req.Body) > 0 {
		bodyBytes, err := json.Marshal(req.Body)
		if err != nil {
			return TestConnectionResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to marshal request body: %v", err),
			}
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(strings.ToUpper(req.Method), req.URL, bodyReader)
	if err != nil {
		return TestConnectionResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create request: %v", err),
		}
	}

	// Set headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set default Content-Type if not provided and body exists
	if req.Method != "GET" && len(req.Body) > 0 && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Execute request
	resp, err := client.Do(httpReq)
	if err != nil {
		return TestConnectionResponse{
			Success:      false,
			ResponseTime: float64(time.Since(start).Nanoseconds()) / 1e6,
			Error:        fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	responseTime := float64(time.Since(start).Nanoseconds()) / 1e6

	// Read response body (limit to first 500 characters for preview)
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 500))
	bodyPreview := ""
	if err == nil {
		bodyPreview = string(bodyBytes)
		if len(bodyBytes) == 500 {
			bodyPreview += "..."
		}
	}

	// Collect response headers
	responseHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			responseHeaders[key] = values[0]
		}
	}

	// Determine success based on status code
	success := resp.StatusCode >= 200 && resp.StatusCode < 400

	return TestConnectionResponse{
		Success:      success,
		StatusCode:   resp.StatusCode,
		ResponseTime: responseTime,
		Headers:      responseHeaders,
		BodyPreview:  bodyPreview,
		Error:        "",
	}
}

func (c *Coordinator) startTestRun(testRun *TestRun) {
	testRun.MarkRunning()

	// Send test plan to all connected agents
	if err := c.broadcastTestStart(testRun); err != nil {
		LogError("Failed to start test run %s: %v", testRun.ID, err)
		testRun.Fail("Failed to broadcast test start")
		return
	}

	// Parse test duration and schedule completion
	duration, err := time.ParseDuration(testRun.TestPlan.Duration)
	if err != nil {
		LogWarn("Invalid duration %s, using 1m", testRun.TestPlan.Duration)
		duration = time.Minute
	}

	// Schedule test completion
	time.AfterFunc(duration, func() {
		c.completeTestRun(testRun.ID)
	})

	LogInfo("Test commands sent to %d agents for test run: %s", len(c.connectedAgents), testRun.Name)
	LogInfo("Test will complete automatically in %s", duration)
}

func (c *Coordinator) stopTestRun(testRun *TestRun) {
	// Send stop command to agents
	stopCommand := TestStartCommand{
		TestRunID: testRun.ID,
		Command:   "STOP",
	}

	data, _ := json.Marshal(stopCommand)
	c.natsConn.Publish("armonite.test.command", data)

	testRun.Status = TestRunStatusCompleting
	LogInfo("Stop command sent for test run: %s", testRun.Name)
}
