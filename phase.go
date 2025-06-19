package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type PhaseOrchestrator struct {
	testRunID string
	testPlan  *TestPlan
	natsConn  *nats.Conn

	// Phase tracking
	currentPhase    int
	phaseStartTime  time.Time
	activeAgents    map[string]*AgentInfo
	completedAgents map[string]bool
	mu              sync.RWMutex

	// Control channels
	stopCh      chan struct{}
	phaseDoneCh chan int
}

func NewPhaseOrchestrator(testRunID string, testPlan *TestPlan, natsConn *nats.Conn, agents map[string]*AgentInfo) *PhaseOrchestrator {
	activeAgents := make(map[string]*AgentInfo)
	for id, agent := range agents {
		activeAgents[id] = agent
	}

	return &PhaseOrchestrator{
		testRunID:       testRunID,
		testPlan:        testPlan,
		natsConn:        natsConn,
		activeAgents:    activeAgents,
		completedAgents: make(map[string]bool),
		stopCh:          make(chan struct{}),
		phaseDoneCh:     make(chan int, 10),
	}
}

func (po *PhaseOrchestrator) Start() {
	if po.testPlan.RampUpStrategy == nil || len(po.testPlan.RampUpStrategy.Phases) == 0 {
		LogInfo("No custom phases defined, using standard ramp-up")
		return
	}

	LogInfo("Starting phase orchestration for test run %s with %d phases",
		po.testRunID, len(po.testPlan.RampUpStrategy.Phases))

	// Start phase completion monitoring
	go po.monitorPhaseCompletion()

	// Execute phases
	go po.executePhases()
}

func (po *PhaseOrchestrator) Stop() {
	select {
	case <-po.stopCh:
		// Already stopped
	default:
		close(po.stopCh)
	}
}

func (po *PhaseOrchestrator) executePhases() {
	defer func() {
		LogInfo("Phase orchestration completed for test run %s", po.testRunID)
	}()

	for phaseIndex, phase := range po.testPlan.RampUpStrategy.Phases {
		select {
		case <-po.stopCh:
			LogInfo("Phase orchestration stopped for test run %s", po.testRunID)
			return
		default:
		}

		po.mu.Lock()
		po.currentPhase = phaseIndex
		po.phaseStartTime = time.Now()
		po.completedAgents = make(map[string]bool)
		po.mu.Unlock()

		phaseID := fmt.Sprintf("%s-phase-%d", po.testRunID, phaseIndex)

		LogInfo("Starting phase %d (%s mode, %d concurrency, %s duration)",
			phaseIndex, phase.Mode, phase.Concurrency, phase.Duration)

		if phase.Mode == "sequential" {
			po.executeSequentialPhase(phaseIndex, phase, phaseID)
		} else {
			po.executeParallelPhase(phaseIndex, phase, phaseID)
		}

		// Wait for phase duration
		phaseDuration, err := time.ParseDuration(phase.Duration)
		if err != nil {
			LogWarn("Invalid phase duration %s, using 30s", phase.Duration)
			phaseDuration = 30 * time.Second
		}

		// Wait for phase completion or timeout
		phaseTimer := time.NewTimer(phaseDuration)
		select {
		case <-phaseTimer.C:
			LogInfo("Phase %d completed by timeout", phaseIndex)
		case <-po.phaseDoneCh:
			LogInfo("Phase %d completed by all agents", phaseIndex)
			phaseTimer.Stop()
		case <-po.stopCh:
			phaseTimer.Stop()
			return
		}
	}
}

func (po *PhaseOrchestrator) executeSequentialPhase(phaseIndex int, phase RampPhase, phaseID string) {
	// Create ordered list of agents for sequential execution
	agentIDs := make([]string, 0, len(po.activeAgents))
	for id := range po.activeAgents {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs) // Deterministic ordering

	if len(agentIDs) == 0 {
		LogWarn("No active agents for sequential phase %d", phaseIndex)
		return
	}

	// Calculate time per agent for sequential execution
	phaseDuration, err := time.ParseDuration(phase.Duration)
	if err != nil {
		phaseDuration = 30 * time.Second
	}
	timePerAgent := phaseDuration / time.Duration(len(agentIDs))

	LogInfo("Sequential phase %d: %d agents, %s per agent",
		phaseIndex, len(agentIDs), timePerAgent)

	// Execute agents sequentially
	for i, agentID := range agentIDs {
		select {
		case <-po.stopCh:
			return
		default:
		}

		// Wait for this agent's turn (except for the first agent)
		if i > 0 {
			time.Sleep(timePerAgent)
		}

		// Send start command to specific agent
		phaseInfo := PhaseInfo{
			PhaseIndex:  phaseIndex,
			PhaseID:     phaseID,
			Mode:        phase.Mode,
			Concurrency: phase.Concurrency,
			Duration:    timePerAgent.String(),
			AgentOrder:  []string{agentID},
		}

		po.sendPhaseCommand(agentID, "START_PHASE", &phaseInfo)

		LogInfo("Sequential phase %d: Started agent %s (%d/%d)",
			phaseIndex, agentID, i+1, len(agentIDs))
	}
}

func (po *PhaseOrchestrator) executeParallelPhase(phaseIndex int, phase RampPhase, phaseID string) {
	// Send start command to all agents simultaneously
	phaseInfo := PhaseInfo{
		PhaseIndex:  phaseIndex,
		PhaseID:     phaseID,
		Mode:        phase.Mode,
		Concurrency: phase.Concurrency,
		Duration:    phase.Duration,
	}

	// Send to all agents
	agentCount := 0
	for agentID := range po.activeAgents {
		po.sendPhaseCommand(agentID, "START_PHASE", &phaseInfo)
		agentCount++
	}

	LogInfo("Parallel phase %d: Started %d agents simultaneously", phaseIndex, agentCount)
}

func (po *PhaseOrchestrator) sendPhaseCommand(agentID string, command string, phase *PhaseInfo) {
	cmd := TestStartCommand{
		TestRunID:    po.testRunID,
		Command:      command,
		CurrentPhase: phase,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		LogError("Failed to marshal phase command: %v", err)
		return
	}

	// Send to agent-specific subject for precise control
	subject := fmt.Sprintf("armonite.agent.%s.command", agentID)
	if err := po.natsConn.Publish(subject, data); err != nil {
		LogError("Failed to send phase command to agent %s: %v", agentID, err)
	}
}

func (po *PhaseOrchestrator) monitorPhaseCompletion() {
	// Subscribe to phase completion messages for this test run
	subject := fmt.Sprintf("armonite.phase.complete.%s", po.testRunID)

	_, err := po.natsConn.Subscribe(subject, func(msg *nats.Msg) {
		var completion PhaseCompletion
		if err := json.Unmarshal(msg.Data, &completion); err != nil {
			LogError("Failed to unmarshal phase completion: %v", err)
			return
		}

		po.mu.Lock()
		defer po.mu.Unlock()

		// Mark agent as completed for current phase
		if completion.PhaseIndex == po.currentPhase {
			po.completedAgents[completion.AgentID] = true

			LogInfo("Agent %s completed phase %d (%d/%d agents done)",
				completion.AgentID, completion.PhaseIndex,
				len(po.completedAgents), len(po.activeAgents))

			// Check if all agents completed this phase
			if len(po.completedAgents) == len(po.activeAgents) {
				LogInfo("All agents completed phase %d", completion.PhaseIndex)
				select {
				case po.phaseDoneCh <- completion.PhaseIndex:
				default:
					// Channel might be full, that's okay
				}
			}
		}
	})

	if err != nil {
		LogError("Failed to subscribe to phase completion: %v", err)
	}
}
