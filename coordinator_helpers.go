package main

import (
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

// handleTelemetryUpdate processes telemetry via internal message passing
func (c *Coordinator) handleTelemetryUpdate(metrics AgentMetrics) {
	// Publish telemetry update as internal message
	telemetryUpdate := map[string]interface{}{
		"type":    "telemetry_update",
		"metrics": metrics,
	}

	data, err := json.Marshal(telemetryUpdate)
	if err != nil {
		LogError("Failed to marshal telemetry update: %v", err)
		return
	}

	// Use internal NATS subject for coordinator state updates
	c.natsConn.Publish("armonite.coordinator.internal", data)
}

// startInternalMessageHandler handles coordinator internal messages
func (c *Coordinator) startInternalMessageHandler() {
	go func() {
		// Local state for this goroutine - no mutexes needed
		agentResults := make(map[string][]AgentResult) // testRunID -> results

		_, err := c.natsConn.Subscribe("armonite.coordinator.internal", func(msg *nats.Msg) {
			var update map[string]interface{}
			if err := json.Unmarshal(msg.Data, &update); err != nil {
				LogError("Failed to unmarshal internal message: %v", err)
				return
			}

			updateType, ok := update["type"].(string)
			if !ok {
				return
			}

			switch updateType {
			case "telemetry_update":
				c.processTelemetryUpdate(update, agentResults)
			case "test_run_started":
				// Clear results for new test run
				testRunID := update["test_run_id"].(string)
				agentResults[testRunID] = make([]AgentResult, 0)
			case "test_run_completed":
				// Archive results
				testRunID := update["test_run_id"].(string)
				if results, exists := agentResults[testRunID]; exists {
					go c.saveAgentResultsToDatabase(testRunID, results)
				}
			case "get_agent_results":
				// Handle request for agent results
				testRunID, _ := update["test_run_id"].(string)
				replyTo, _ := update["reply_to"].(string)

				results := []AgentResult{}
				if testRunID != "" && replyTo != "" {
					if agentRes, exists := agentResults[testRunID]; exists {
						results = agentRes
					}
				}

				response := map[string]interface{}{
					"type":    "agent_results_response",
					"results": results,
				}

				if responseData, err := json.Marshal(response); err == nil {
					c.natsConn.Publish(replyTo, responseData)
				}
			}
		})

		if err != nil {
			LogError("Failed to subscribe to internal messages: %v", err)
		}

		// Keep this goroutine alive
		select {}
	}()
}

func (c *Coordinator) processTelemetryUpdate(update map[string]interface{}, agentResults map[string][]AgentResult) {
	metricsData, ok := update["metrics"]
	if !ok {
		return
	}

	// Convert back to AgentMetrics struct
	metricsBytes, err := json.Marshal(metricsData)
	if err != nil {
		return
	}

	var metrics AgentMetrics
	if err := json.Unmarshal(metricsBytes, &metrics); err != nil {
		return
	}

	// Get current test run ID via NATS query (non-blocking)
	c.getCurrentTestRunInfo(func(testRunID, region string) {
		if testRunID == "" {
			return
		}

		// Initialize results slice for this test run if needed
		if _, exists := agentResults[testRunID]; !exists {
			agentResults[testRunID] = make([]AgentResult, 0)
		}

		// Create new agent result
		newResult := AgentResult{
			AgentID:      metrics.AgentID,
			Region:       region,
			Requests:     metrics.Requests,
			Errors:       metrics.Errors,
			AvgLatencyMs: metrics.AvgLatencyMs,
			MinLatencyMs: metrics.MinLatencyMs,
			MaxLatencyMs: metrics.MaxLatencyMs,
			StatusCodes:  metrics.StatusCodes,
		}

		// Find existing agent result or append new one
		found := false
		for i, agent := range agentResults[testRunID] {
			if agent.AgentID == metrics.AgentID {
				agentResults[testRunID][i] = newResult
				found = true
				break
			}
		}

		if !found {
			agentResults[testRunID] = append(agentResults[testRunID], newResult)
		}

		// Periodically save to database (async)
		go c.saveAgentResultsToDatabase(testRunID, agentResults[testRunID])
	})
}

func (c *Coordinator) getCurrentTestRunInfo(callback func(testRunID, region string)) {
	// This would normally query via NATS, but for now use a minimal lock
	c.mu.RLock()
	testRunID := ""
	if c.currentTestRun != nil {
		testRunID = c.currentTestRun.ID
	}
	c.mu.RUnlock()

	callback(testRunID, "")
}

// getAgentResultsViaMessage retrieves agent results using message passing
func (c *Coordinator) getAgentResultsViaMessage(testRunID string, callback func([]AgentResult)) {
	// Send request for agent results
	request := map[string]interface{}{
		"type":        "get_agent_results",
		"test_run_id": testRunID,
		"reply_to":    "armonite.coordinator.response." + testRunID,
	}

	data, err := json.Marshal(request)
	if err != nil {
		callback([]AgentResult{})
		return
	}

	// Subscribe to response
	responseSub, err := c.natsConn.SubscribeSync("armonite.coordinator.response." + testRunID)
	if err != nil {
		callback([]AgentResult{})
		return
	}
	defer responseSub.Unsubscribe()

	// Send request
	c.natsConn.Publish("armonite.coordinator.internal", data)

	// Wait for response with timeout
	msg, err := responseSub.NextMsg(2 * time.Second)
	if err != nil {
		callback([]AgentResult{})
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(msg.Data, &response); err != nil {
		callback([]AgentResult{})
		return
	}

	if resultsData, ok := response["results"]; ok {
		// Convert to AgentResult slice
		resultsBytes, _ := json.Marshal(resultsData)
		var results []AgentResult
		json.Unmarshal(resultsBytes, &results)
		callback(results)
	} else {
		callback([]AgentResult{})
	}
}

func (c *Coordinator) saveAgentResultsToDatabase(testRunID string, results []AgentResult) {
	if c.database != nil {
		if err := c.database.SaveAgentResults(testRunID, results); err != nil {
			LogError("Failed to save agent results to database: %v", err)
		}
	}
}
