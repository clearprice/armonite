package main

import (
	"time"

	"github.com/google/uuid"
)

type TestRun struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	TestPlan    TestPlan               `json:"test_plan"`
	Status      TestRunStatus          `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Duration    *string                `json:"duration,omitempty"`
	Results     *TestRunResults        `json:"results,omitempty"`
	AgentCount  int                    `json:"agent_count"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type TestRunStatus string

const (
	TestRunStatusCreated    TestRunStatus = "created"
	TestRunStatusWaiting    TestRunStatus = "waiting_for_agents"
	TestRunStatusRunning    TestRunStatus = "running"
	TestRunStatusCompleting TestRunStatus = "completing"
	TestRunStatusCompleted  TestRunStatus = "completed"
	TestRunStatusFailed     TestRunStatus = "failed"
	TestRunStatusCancelled  TestRunStatus = "cancelled"
)

type TestRunResults struct {
	TotalRequests  int64            `json:"total_requests"`
	TotalErrors    int64            `json:"total_errors"`
	SuccessRate    float64          `json:"success_rate"`
	AvgLatencyMs   float64          `json:"avg_latency_ms"`
	MinLatencyMs   float64          `json:"min_latency_ms"`
	MaxLatencyMs   float64          `json:"max_latency_ms"`
	RequestsPerSec float64          `json:"requests_per_sec"`
	StatusCodes    map[string]int64 `json:"status_codes"`
	AgentResults   []AgentResult    `json:"agent_results"`
}

type CreateTestRunRequest struct {
	Name       string                 `json:"name"`
	TestPlan   TestPlan               `json:"test_plan"`
	MinAgents  int                    `json:"min_agents"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type StartTestRunRequest struct {
	TestRunID string `json:"test_run_id"`
}

func NewTestRun(name string, testPlan TestPlan, minAgents int, parameters map[string]interface{}) *TestRun {
	return &TestRun{
		ID:         uuid.New().String(),
		Name:       name,
		TestPlan:   testPlan,
		Status:     TestRunStatusCreated,
		CreatedAt:  time.Now(),
		AgentCount: minAgents,
		Parameters: parameters,
	}
}

func (tr *TestRun) Start() {
	now := time.Now()
	tr.StartedAt = &now
	tr.Status = TestRunStatusWaiting
}

func (tr *TestRun) MarkRunning() {
	tr.Status = TestRunStatusRunning
}

func (tr *TestRun) Complete(results *TestRunResults) {
	now := time.Now()
	tr.CompletedAt = &now
	tr.Status = TestRunStatusCompleted
	tr.Results = results

	if tr.StartedAt != nil {
		duration := now.Sub(*tr.StartedAt).String()
		tr.Duration = &duration
	}
}

func (tr *TestRun) Fail(reason string) {
	now := time.Now()
	tr.CompletedAt = &now
	tr.Status = TestRunStatusFailed

	if tr.StartedAt != nil {
		duration := now.Sub(*tr.StartedAt).String()
		tr.Duration = &duration
	}
}

func (tr *TestRun) Cancel() {
	now := time.Now()
	tr.CompletedAt = &now
	tr.Status = TestRunStatusCancelled

	if tr.StartedAt != nil {
		duration := now.Sub(*tr.StartedAt).String()
		tr.Duration = &duration
	}
}
