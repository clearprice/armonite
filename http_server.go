package main

import (
	"context"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type CoordinatorStatus struct {
	Status        string            `json:"status"`
	Uptime        string            `json:"uptime"`
	Host          string            `json:"host"`
	HTTPPort      int               `json:"http_port"`
	TestPlan      *TestPlan         `json:"test_plan,omitempty"`
	TestRunning   bool              `json:"test_running"`
	TestCompleted bool              `json:"test_completed"`
	StartTime     *time.Time        `json:"start_time,omitempty"`
	Agents        []AgentStatusInfo `json:"agents"`
	TotalRequests int64             `json:"total_requests"`
	TotalErrors   int64             `json:"total_errors"`
	SuccessRate   float64           `json:"success_rate"`
}

type AgentStatusInfo struct {
	ID             string    `json:"id"`
	Region         string    `json:"region"`
	Concurrency    int       `json:"concurrency"`
	ConnectedAt    time.Time `json:"connected_at"`
	LastSeen       time.Time `json:"last_seen"`
	Requests       int64     `json:"requests"`
	Errors         int64     `json:"errors"`
	AvgLatency     float64   `json:"avg_latency_ms"`
	Status         string    `json:"status"`
	ExecutionState string    `json:"execution_state"`
}

func (c *Coordinator) startHTTPServer() {
	// Start API server
	go c.startAPIServer()

	// Start UI server if enabled
	if c.config.Server.EnableUI {
		go c.startUIServer()
	}
}

func (c *Coordinator) startAPIServer() {
	// Set Gin to release mode to reduce logs
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery()) // Only use recovery middleware, no request logging

	// Add timeout middleware to prevent hanging connections
	router.Use(func(ctx *gin.Context) {
		// Set a timeout for API requests
		timeoutCtx, cancel := context.WithTimeout(ctx.Request.Context(), 30*time.Second)
		defer cancel()
		ctx.Request = ctx.Request.WithContext(timeoutCtx)
		ctx.Next()
	})

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// API endpoints
	api := router.Group("/api/v1")
	{
		// Coordinator status (not test status)
		api.GET("/status", c.handleCoordinatorStatus)
		api.GET("/agents", c.handleAgents)

		// Test run management
		api.POST("/test-runs", c.handleCreateTestRun)
		api.GET("/test-runs", c.handleListTestRuns)
		api.GET("/test-runs/:id", c.handleGetTestRun)
		api.GET("/test-runs/:id/results", c.handleGetTestRunResults)
		api.POST("/test-runs/:id/start", c.handleStartTestRun)
		api.POST("/test-runs/:id/stop", c.handleStopTestRun)
		api.POST("/test-runs/:id/rerun", c.handleRerunTestRun)
		api.DELETE("/test-runs/:id", c.handleDeleteTestRun)

		// Bulk deletion endpoints
		api.DELETE("/test-runs", c.handleBulkDeleteTestRuns)
		api.GET("/test-runs/stats", c.handleTestRunStats)

		// Test connection endpoint
		api.POST("/test-connection", c.handleTestConnection)

		// Legacy endpoints (deprecated)
		api.GET("/metrics", c.handleMetrics)
	}

	// Root endpoint - basic info
	router.GET("/", c.handleRootInfo)

	// Health check
	router.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Use configured HTTP port
	apiPort := c.config.Server.HTTPPort
	if apiPort == 0 {
		apiPort = 8080 // Default fallback
	}

	LogDebug("API server starting on port %d", apiPort)

	// Create HTTP server with better configuration for high load
	server := &http.Server{
		Addr:         ":" + strconv.Itoa(apiPort),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		// Increase max header size for large requests
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		LogError("Failed to start API server: %v", err)
	}
}

func (c *Coordinator) startUIServer() {
	// Set Gin to release mode to reduce logs
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery()) // Only use recovery middleware, no request logging

	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Get embedded UI filesystem
	uiFS, err := GetEmbeddedUI()
	if err != nil {
		LogError("Failed to get embedded UI filesystem: %v", err)
		return
	}

	// Serve React static assets from embedded filesystem
	assetsFS, err := fs.Sub(uiFS, "assets")
	if err != nil {
		LogError("Failed to get assets subdirectory: %v", err)
		return
	}
	router.StaticFS("/assets", http.FS(assetsFS))

	// Serve the React app for all routes (SPA routing)
	router.NoRoute(func(ctx *gin.Context) {
		// Read index.html from embedded filesystem
		indexHTML, err := fs.ReadFile(uiFS, "index.html")
		if err != nil {
			LogError("Failed to read embedded index.html: %v", err)
			ctx.String(http.StatusInternalServerError, "UI not available")
			return
		}

		ctx.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})

	// Use UI port (API port + 1)
	uiPort := c.config.Server.HTTPPort + 1
	if uiPort == 1 {
		uiPort = 8081 // Default fallback if HTTPPort was 0
	}

	LogDebug("UI server starting on port %d", uiPort)
	LogInfo("React UI served from embedded filesystem")

	if err := router.Run(":" + strconv.Itoa(uiPort)); err != nil {
		LogError("Failed to start UI server: %v", err)
	}
}

func (c *Coordinator) handleCoordinatorStatus(ctx *gin.Context) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	uptime := time.Since(time.Now().Add(-time.Hour)) // Simple uptime calculation

	status := gin.H{
		"service":          "armonite-coordinator",
		"status":           "running",
		"uptime":           uptime.String(),
		"host":             c.host,
		"nats_port":        c.port,
		"http_port":        c.config.Server.HTTPPort,
		"connected_agents": len(c.connectedAgents),
		"total_test_runs":  len(c.testRuns),
	}

	// Add current test run info if one is active
	if c.currentTestRun != nil {
		status["current_test_run"] = gin.H{
			"id":     c.currentTestRun.ID,
			"name":   c.currentTestRun.Name,
			"status": c.currentTestRun.Status,
		}
	}

	ctx.JSON(http.StatusOK, status)
}

func (c *Coordinator) handleAgents(ctx *gin.Context) {
	status := c.getCoordinatorStatus()
	ctx.JSON(http.StatusOK, gin.H{"agents": status.Agents})
}

func (c *Coordinator) handleMetrics(ctx *gin.Context) {
	status := c.getCoordinatorStatus()
	metrics := gin.H{
		"total_requests": status.TotalRequests,
		"total_errors":   status.TotalErrors,
		"success_rate":   status.SuccessRate,
		"agent_count":    len(status.Agents),
	}
	ctx.JSON(http.StatusOK, metrics)
}

func (c *Coordinator) handleRootInfo(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"service": "armonite-coordinator",
		"version": "1.0.0",
		"endpoints": map[string]interface{}{
			"coordinator": []string{
				"GET /api/v1/status",
				"GET /api/v1/agents",
				"GET /health",
			},
			"test_runs": []string{
				"POST /api/v1/test-runs",
				"GET /api/v1/test-runs",
				"GET /api/v1/test-runs/{id}",
				"GET /api/v1/test-runs/{id}/results",
				"POST /api/v1/test-runs/{id}/start",
				"POST /api/v1/test-runs/{id}/stop",
				"POST /api/v1/test-runs/{id}/rerun",
				"DELETE /api/v1/test-runs/{id}",
				"DELETE /api/v1/test-runs",
				"GET /api/v1/test-runs/stats",
				"POST /api/v1/test-connection",
			},
		},
		"documentation": "Distributed load testing coordinator with test run management",
	})
}

func (c *Coordinator) getCoordinatorStatus() CoordinatorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	uptime := time.Hour // Default uptime placeholder

	// Collect agent status
	agents := make([]AgentStatusInfo, 0, len(c.connectedAgents))
	for _, agentInfo := range c.connectedAgents {
		status := "connected"
		if time.Since(agentInfo.LastSeen) > 30*time.Second {
			status = "stale"
		}

		// Determine execution state
		executionState := "idle"
		if agentInfo.ExecutionState != "" {
			executionState = agentInfo.ExecutionState
		} else if c.currentTestRun != nil && c.currentTestRun.Status == TestRunStatusRunning {
			// If test is running but no specific execution state, assume running
			executionState = "running"
		}

		// Get metrics for this agent from current test run
		var requests, errors int64
		var avgLatency float64
		if c.currentTestRun != nil {
			if results, exists := c.agentResults[c.currentTestRun.ID]; exists {
				for _, result := range results {
					if result.AgentID == agentInfo.ID {
						requests = result.Requests
						errors = result.Errors
						avgLatency = result.AvgLatencyMs
						break
					}
				}
			}
		}

		agents = append(agents, AgentStatusInfo{
			ID:             agentInfo.ID,
			Region:         agentInfo.Region,
			Concurrency:    agentInfo.Concurrency,
			ConnectedAt:    agentInfo.ConnectedAt,
			LastSeen:       agentInfo.LastSeen,
			Requests:       requests,
			Errors:         errors,
			AvgLatency:     avgLatency,
			Status:         status,
			ExecutionState: executionState,
		})
	}

	// Calculate totals from current test run
	var totalRequests, totalErrors int64
	if c.currentTestRun != nil {
		if results, exists := c.agentResults[c.currentTestRun.ID]; exists {
			for _, result := range results {
				totalRequests += result.Requests
				totalErrors += result.Errors
			}
		}
	}

	successRate := float64(0)
	if totalRequests > 0 {
		successRate = float64(totalRequests-totalErrors) / float64(totalRequests) * 100
	}

	// Determine coordinator status based on test runs and agents
	coordinatorStatus := "waiting"
	var testRunning, testCompleted bool
	var startTime *time.Time

	if c.currentTestRun != nil {
		switch c.currentTestRun.Status {
		case TestRunStatusRunning:
			coordinatorStatus = "running"
			testRunning = true
			startTime = c.currentTestRun.StartedAt
		case TestRunStatusCompleted, TestRunStatusFailed, TestRunStatusCancelled:
			coordinatorStatus = "completed"
			testCompleted = true
			startTime = c.currentTestRun.StartedAt
		case TestRunStatusWaiting:
			coordinatorStatus = "waiting_for_agents"
		}
	} else if len(c.connectedAgents) > 0 {
		coordinatorStatus = "ready"
	}

	return CoordinatorStatus{
		Status:        coordinatorStatus,
		Uptime:        uptime.Round(time.Second).String(),
		Host:          c.host,
		HTTPPort:      c.config.Server.HTTPPort,
		TestPlan:      nil, // Test plans are now part of test runs
		TestRunning:   testRunning,
		TestCompleted: testCompleted,
		StartTime:     startTime,
		Agents:        agents,
		TotalRequests: totalRequests,
		TotalErrors:   totalErrors,
		SuccessRate:   successRate,
	}
}
