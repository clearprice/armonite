package main

import (
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

type AgentRegistration struct {
	AgentID     string `json:"agent_id"`
	Region      string `json:"region"`
	Concurrency int    `json:"concurrency"`
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	Action      string `json:"action"` // "register", "unregister"
}

type AgentHeartbeat struct {
	AgentID   string `json:"agent_id"`
	Timestamp string `json:"timestamp"`
}

type AgentExecutionUpdate struct {
	AgentID   string `json:"agent_id"`
	Status    string `json:"status"` // "starting", "running", "stopping", "completed"
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func (c *Coordinator) startAgentRegistration() {
	// Subscribe to agent registration messages
	_, err := c.natsConn.Subscribe("armonite.agent.register", func(msg *nats.Msg) {
		var registration AgentRegistration
		if err := json.Unmarshal(msg.Data, &registration); err != nil {
			LogError("Failed to unmarshal agent registration: %v", err)
			return
		}

		switch registration.Action {
		case "register":
			c.handleAgentRegistration(registration)
		case "unregister":
			c.handleAgentUnregistration(registration)
		}
	})
	if err != nil {
		LogError("Failed to subscribe to agent registration: %v", err)
		return
	}

	// Subscribe to agent heartbeats
	_, err = c.natsConn.Subscribe("armonite.agent.heartbeat", func(msg *nats.Msg) {
		var heartbeat AgentHeartbeat
		if err := json.Unmarshal(msg.Data, &heartbeat); err != nil {
			LogError("Failed to unmarshal agent heartbeat: %v", err)
			return
		}

		c.handleAgentHeartbeat(heartbeat)
	})
	if err != nil {
		LogError("Failed to subscribe to agent heartbeat: %v", err)
		return
	}

	// Subscribe to agent execution updates
	_, err = c.natsConn.Subscribe("armonite.agent.execution", func(msg *nats.Msg) {
		var update AgentExecutionUpdate
		if err := json.Unmarshal(msg.Data, &update); err != nil {
			LogError("Failed to unmarshal execution update: %v", err)
			return
		}

		c.handleAgentExecutionUpdate(update)
	})
	if err != nil {
		LogError("Failed to subscribe to agent execution updates: %v", err)
		return
	}

	// Start agent cleanup routine for stale agents
	go c.startAgentCleanup()

	LogInfo("Agent registration system ready")
}

func (c *Coordinator) handleAgentRegistration(registration AgentRegistration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	agentInfo := &AgentInfo{
		ID:          registration.AgentID,
		Region:      registration.Region,
		ConnectedAt: now,
		LastSeen:    now,
		Concurrency: registration.Concurrency,
	}

	// Initialize ramp-up execution if there's a current test run with ramp-up strategy
	if c.currentTestRun != nil && c.currentTestRun.TestPlan.RampUpStrategy != nil {
		calculator, err := NewRampUpCalculator(*c.currentTestRun.TestPlan.RampUpStrategy, registration.Concurrency)
		if err == nil {
			agentInfo.RampUpExecution = calculator.Start()
		}
	}

	// Check if this is a new agent
	_, exists := c.connectedAgents[registration.AgentID]
	c.connectedAgents[registration.AgentID] = agentInfo

	if !exists {
		LogInfo("Agent registered: %s (region: %s, concurrency: %d)",
			registration.AgentID, registration.Region, registration.Concurrency)
		LogInfo("Total connected agents: %d", len(c.connectedAgents))

		// Check if there's a waiting test run that can now start
		if c.currentTestRun != nil && c.currentTestRun.Status == TestRunStatusWaiting {
			if len(c.connectedAgents) >= c.currentTestRun.AgentCount {
				LogInfo("Starting waiting test run: %s (%d agents now available)",
					c.currentTestRun.Name, len(c.connectedAgents))
				go c.startTestRun(c.currentTestRun)
			} else {
				LogInfo("Test run still waiting for agents: %s (%d/%d connected)",
					c.currentTestRun.Name, len(c.connectedAgents), c.currentTestRun.AgentCount)
			}
		} else if c.currentTestRun == nil {
			LogInfo("Agent registered - coordinator idle, waiting for test run")
		} else {
			LogInfo("Agent joined during active test: %s", registration.AgentID)
		}
	} else {
		LogDebug("Agent re-registered: %s", registration.AgentID)
	}
}

func (c *Coordinator) handleAgentUnregistration(registration AgentRegistration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.connectedAgents[registration.AgentID]; exists {
		delete(c.connectedAgents, registration.AgentID)
		LogInfo("Agent unregistered: %s", registration.AgentID)
		LogInfo("Total connected agents: %d", len(c.connectedAgents))
	}
}

func (c *Coordinator) handleAgentHeartbeat(heartbeat AgentHeartbeat) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if agent, exists := c.connectedAgents[heartbeat.AgentID]; exists {
		agent.LastSeen = time.Now()
		LogDebug("Heartbeat from agent: %s", heartbeat.AgentID)
	}
}

func (c *Coordinator) handleAgentExecutionUpdate(update AgentExecutionUpdate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update agent execution state
	if agent, exists := c.connectedAgents[update.AgentID]; exists {
		agent.ExecutionState = update.Status
	}

	switch update.Status {
	case "starting":
		LogInfo("Agent %s: %s", update.AgentID, update.Message)
	case "running":
		LogInfo("Agent %s: %s", update.AgentID, update.Message)
	case "stopping":
		LogInfo("Agent %s: %s", update.AgentID, update.Message)
	case "completed":
		LogInfo("Agent %s: %s", update.AgentID, update.Message)
		// Reset to idle when completed
		if agent, exists := c.connectedAgents[update.AgentID]; exists {
			agent.ExecutionState = "idle"
		}
	default:
		LogInfo("Agent %s: %s", update.AgentID, update.Message)
	}
}

func (c *Coordinator) startAgentCleanup() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanupStaleAgents()
	}
}

func (c *Coordinator) cleanupStaleAgents() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	staleThreshold := 60 * time.Second // Consider agents stale after 60 seconds

	for agentID, agent := range c.connectedAgents {
		if now.Sub(agent.LastSeen) > staleThreshold {
			delete(c.connectedAgents, agentID)
			LogWarn("Removed stale agent: %s (last seen: %s)",
				agentID, agent.LastSeen.Format("15:04:05"))
			LogInfo("Total connected agents: %d", len(c.connectedAgents))
		}
	}
}
