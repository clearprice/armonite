package main

import (
	"encoding/json"
	"time"
)

func (a *Agent) registerWithCoordinator() error {
	registration := AgentRegistration{
		AgentID:     a.id,
		Region:      a.region,
		Concurrency: a.concurrency,
		Status:      "ready",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Action:      "register",
	}

	data, err := json.Marshal(registration)
	if err != nil {
		return err
	}

	err = a.natsConn.Publish("armonite.agent.register", data)
	if err != nil {
		return err
	}

	LogInfo("Registered with coordinator")
	return nil
}

func (a *Agent) unregisterFromCoordinator() {
	registration := AgentRegistration{
		AgentID:     a.id,
		Region:      a.region,
		Concurrency: a.concurrency,
		Status:      "shutting_down",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Action:      "unregister",
	}

	data, err := json.Marshal(registration)
	if err != nil {
		LogError("Failed to marshal unregistration: %v", err)
		return
	}

	if err := a.natsConn.Publish("armonite.agent.register", data); err != nil {
		LogError("Failed to unregister: %v", err)
	} else {
		LogInfo("Unregistered from coordinator")
	}
}

func (a *Agent) startHeartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			a.sendHeartbeat()
		}
	}()
}

func (a *Agent) sendHeartbeat() {
	heartbeat := AgentHeartbeat{
		AgentID:   a.id,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		LogError("Failed to marshal heartbeat: %v", err)
		return
	}

	if err := a.natsConn.Publish("armonite.agent.heartbeat", data); err != nil {
		LogDebug("Failed to send heartbeat: %v", err)
	} else {
		LogDebug("Sent heartbeat to coordinator")
	}
}
