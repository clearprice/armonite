package main

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database models
type DBTestRun struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"not null" json:"name"`
	TestPlan    string     `gorm:"type:text" json:"test_plan"` // JSON serialized
	Status      string     `gorm:"not null" json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Duration    *string    `json:"duration,omitempty"`
	AgentCount  int        `json:"agent_count"`
	Parameters  string     `gorm:"type:text" json:"parameters"` // JSON serialized
	Results     string     `gorm:"type:text" json:"results"`    // JSON serialized
}

type DBAgentResult struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TestRunID    string    `gorm:"not null;index" json:"test_run_id"`
	AgentID      string    `gorm:"not null" json:"agent_id"`
	Region       string    `json:"region"`
	Requests     int64     `json:"requests"`
	Errors       int64     `json:"errors"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	MinLatencyMs float64   `json:"min_latency_ms"`
	MaxLatencyMs float64   `json:"max_latency_ms"`
	StatusCodes  string    `gorm:"type:text" json:"status_codes"` // JSON serialized
	UpdatedAt    time.Time `json:"updated_at"`
}

type Database struct {
	db *gorm.DB
}

func NewDatabase(config DatabaseConfig) (*Database, error) {
	// Configure GORM logger based on global log level
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	// Open SQLite database
	db, err := gorm.Open(sqlite.Open(config.DSN), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(config.MaxOpen)
	sqlDB.SetMaxIdleConns(config.MaxIdle)

	if config.MaxLifetime != "" {
		if lifetime, err := time.ParseDuration(config.MaxLifetime); err == nil {
			sqlDB.SetConnMaxLifetime(lifetime)
		}
	}

	database := &Database{db: db}

	// Run migrations
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	LogInfo("Database initialized successfully with DSN: %s", config.DSN)
	return database, nil
}

func (d *Database) migrate() error {
	return d.db.AutoMigrate(&DBTestRun{}, &DBAgentResult{})
}

func (d *Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Test Run operations
func (d *Database) SaveTestRun(testRun *TestRun) error {
	// Convert TestRun to DBTestRun
	testPlanJSON, err := json.Marshal(testRun.TestPlan)
	if err != nil {
		return fmt.Errorf("failed to marshal test plan: %w", err)
	}

	parametersJSON, err := json.Marshal(testRun.Parameters)
	if err != nil {
		return fmt.Errorf("failed to marshal parameters: %w", err)
	}

	resultsJSON := ""
	if testRun.Results != nil {
		resultsBytes, err := json.Marshal(testRun.Results)
		if err != nil {
			return fmt.Errorf("failed to marshal results: %w", err)
		}
		resultsJSON = string(resultsBytes)
	}

	dbTestRun := DBTestRun{
		ID:          testRun.ID,
		Name:        testRun.Name,
		TestPlan:    string(testPlanJSON),
		Status:      string(testRun.Status),
		CreatedAt:   testRun.CreatedAt,
		StartedAt:   testRun.StartedAt,
		CompletedAt: testRun.CompletedAt,
		Duration:    testRun.Duration,
		AgentCount:  testRun.AgentCount,
		Parameters:  string(parametersJSON),
		Results:     resultsJSON,
	}

	return d.db.Save(&dbTestRun).Error
}

func (d *Database) GetTestRun(id string) (*TestRun, error) {
	var dbTestRun DBTestRun
	if err := d.db.First(&dbTestRun, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return d.convertDBTestRun(&dbTestRun)
}

func (d *Database) ListTestRuns(limit, offset int) ([]*TestRun, error) {
	var dbTestRuns []DBTestRun
	query := d.db.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&dbTestRuns).Error; err != nil {
		return nil, err
	}

	testRuns := make([]*TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRun, err := d.convertDBTestRun(&dbTestRun)
		if err != nil {
			return nil, err
		}
		testRuns[i] = testRun
	}

	return testRuns, nil
}

func (d *Database) DeleteTestRun(id string) error {
	// Delete agent results first
	if err := d.db.Delete(&DBAgentResult{}, "test_run_id = ?", id).Error; err != nil {
		return err
	}
	// Delete test run
	return d.db.Delete(&DBTestRun{}, "id = ?", id).Error
}

// DeleteTestRunsByStatus deletes all test runs with the specified status
func (d *Database) DeleteTestRunsByStatus(status string) (int64, error) {
	// First, get all test run IDs with this status
	var testRunIDs []string
	if err := d.db.Model(&DBTestRun{}).Where("status = ?", status).Pluck("id", &testRunIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to get test run IDs: %w", err)
	}

	if len(testRunIDs) == 0 {
		return 0, nil
	}

	// Delete agent results for these test runs
	if err := d.db.Delete(&DBAgentResult{}, "test_run_id IN ?", testRunIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to delete agent results: %w", err)
	}

	// Delete test runs
	result := d.db.Delete(&DBTestRun{}, "status = ?", status)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete test runs: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// DeleteTestRunsOlderThan deletes test runs older than the specified duration
func (d *Database) DeleteTestRunsOlderThan(duration time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-duration)

	// First, get all test run IDs older than cutoff
	var testRunIDs []string
	if err := d.db.Model(&DBTestRun{}).Where("created_at < ?", cutoffTime).Pluck("id", &testRunIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to get old test run IDs: %w", err)
	}

	if len(testRunIDs) == 0 {
		return 0, nil
	}

	// Delete agent results for these test runs
	if err := d.db.Delete(&DBAgentResult{}, "test_run_id IN ?", testRunIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to delete agent results: %w", err)
	}

	// Delete test runs
	result := d.db.Delete(&DBTestRun{}, "created_at < ?", cutoffTime)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete old test runs: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// DeleteAgentResults deletes agent results for a specific test run
func (d *Database) DeleteAgentResults(testRunID string) error {
	return d.db.Delete(&DBAgentResult{}, "test_run_id = ?", testRunID).Error
}

// GetTestRunCount returns the total number of test runs
func (d *Database) GetTestRunCount() (int64, error) {
	var count int64
	err := d.db.Model(&DBTestRun{}).Count(&count).Error
	return count, err
}

// GetTestRunCountByStatus returns the count of test runs by status
func (d *Database) GetTestRunCountByStatus(status string) (int64, error) {
	var count int64
	err := d.db.Model(&DBTestRun{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

// Agent Result operations
func (d *Database) SaveAgentResults(testRunID string, results []AgentResult) error {
	// Delete existing results for this test run
	if err := d.db.Delete(&DBAgentResult{}, "test_run_id = ?", testRunID).Error; err != nil {
		return err
	}

	// Insert new results
	for _, result := range results {
		statusCodesJSON, err := json.Marshal(result.StatusCodes)
		if err != nil {
			return fmt.Errorf("failed to marshal status codes: %w", err)
		}

		dbResult := DBAgentResult{
			TestRunID:    testRunID,
			AgentID:      result.AgentID,
			Region:       result.Region,
			Requests:     result.Requests,
			Errors:       result.Errors,
			AvgLatencyMs: result.AvgLatencyMs,
			MinLatencyMs: result.MinLatencyMs,
			MaxLatencyMs: result.MaxLatencyMs,
			StatusCodes:  string(statusCodesJSON),
			UpdatedAt:    time.Now(),
		}

		if err := d.db.Create(&dbResult).Error; err != nil {
			return err
		}
	}

	return nil
}

func (d *Database) GetAgentResults(testRunID string) ([]AgentResult, error) {
	var dbResults []DBAgentResult
	if err := d.db.Find(&dbResults, "test_run_id = ?", testRunID).Error; err != nil {
		return nil, err
	}

	results := make([]AgentResult, len(dbResults))
	for i, dbResult := range dbResults {
		var statusCodes map[string]int64
		if err := json.Unmarshal([]byte(dbResult.StatusCodes), &statusCodes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal status codes: %w", err)
		}

		results[i] = AgentResult{
			AgentID:      dbResult.AgentID,
			Region:       dbResult.Region,
			Requests:     dbResult.Requests,
			Errors:       dbResult.Errors,
			AvgLatencyMs: dbResult.AvgLatencyMs,
			MinLatencyMs: dbResult.MinLatencyMs,
			MaxLatencyMs: dbResult.MaxLatencyMs,
			StatusCodes:  statusCodes,
		}
	}

	return results, nil
}

// Helper function to convert DBTestRun to TestRun
func (d *Database) convertDBTestRun(dbTestRun *DBTestRun) (*TestRun, error) {
	var testPlan TestPlan
	if err := json.Unmarshal([]byte(dbTestRun.TestPlan), &testPlan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal test plan: %w", err)
	}

	var parameters map[string]interface{}
	if dbTestRun.Parameters != "" {
		if err := json.Unmarshal([]byte(dbTestRun.Parameters), &parameters); err != nil {
			return nil, fmt.Errorf("failed to unmarshal parameters: %w", err)
		}
	}

	var results *TestRunResults
	if dbTestRun.Results != "" {
		results = &TestRunResults{}
		if err := json.Unmarshal([]byte(dbTestRun.Results), results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal results: %w", err)
		}
	}

	return &TestRun{
		ID:          dbTestRun.ID,
		Name:        dbTestRun.Name,
		TestPlan:    testPlan,
		Status:      TestRunStatus(dbTestRun.Status),
		CreatedAt:   dbTestRun.CreatedAt,
		StartedAt:   dbTestRun.StartedAt,
		CompletedAt: dbTestRun.CompletedAt,
		Duration:    dbTestRun.Duration,
		Results:     results,
		AgentCount:  dbTestRun.AgentCount,
		Parameters:  parameters,
	}, nil
}
