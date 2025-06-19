package main

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type TestResults struct {
	TestName       string           `json:"test_name" xml:"test_name" yaml:"test_name"`
	StartTime      time.Time        `json:"start_time" xml:"start_time" yaml:"start_time"`
	EndTime        time.Time        `json:"end_time" xml:"end_time" yaml:"end_time"`
	Duration       string           `json:"duration" xml:"duration" yaml:"duration"`
	TotalRequests  int64            `json:"total_requests" xml:"total_requests" yaml:"total_requests"`
	TotalErrors    int64            `json:"total_errors" xml:"total_errors" yaml:"total_errors"`
	SuccessRate    float64          `json:"success_rate" xml:"success_rate" yaml:"success_rate"`
	AvgLatencyMs   float64          `json:"avg_latency_ms" xml:"avg_latency_ms" yaml:"avg_latency_ms"`
	MinLatencyMs   float64          `json:"min_latency_ms" xml:"min_latency_ms" yaml:"min_latency_ms"`
	MaxLatencyMs   float64          `json:"max_latency_ms" xml:"max_latency_ms" yaml:"max_latency_ms"`
	RequestsPerSec float64          `json:"requests_per_sec" xml:"requests_per_sec" yaml:"requests_per_sec"`
	StatusCodes    map[string]int64 `json:"status_codes" xml:"status_codes" yaml:"status_codes"`
	Agents         []AgentResult    `json:"agents" xml:"agents" yaml:"agents"`
	Summary        TestSummary      `json:"summary" xml:"summary" yaml:"summary"`
}

type AgentResult struct {
	AgentID      string           `json:"agent_id" xml:"agent_id" yaml:"agent_id"`
	Region       string           `json:"region" xml:"region" yaml:"region"`
	Requests     int64            `json:"requests" xml:"requests" yaml:"requests"`
	Errors       int64            `json:"errors" xml:"errors" yaml:"errors"`
	AvgLatencyMs float64          `json:"avg_latency_ms" xml:"avg_latency_ms" yaml:"avg_latency_ms"`
	MinLatencyMs float64          `json:"min_latency_ms" xml:"min_latency_ms" yaml:"min_latency_ms"`
	MaxLatencyMs float64          `json:"max_latency_ms" xml:"max_latency_ms" yaml:"max_latency_ms"`
	StatusCodes  map[string]int64 `json:"status_codes" xml:"status_codes" yaml:"status_codes"`
}

type TestSummary struct {
	ConfigUsed        TestPlan `json:"config_used" xml:"config_used" yaml:"config_used"`
	ActualConcurrency int      `json:"actual_concurrency" xml:"actual_concurrency" yaml:"actual_concurrency"`
	AgentCount        int      `json:"agent_count" xml:"agent_count" yaml:"agent_count"`
}

type ResultWriter struct {
	config  OutputConfig
	results *TestResults
}

func NewResultWriter(config OutputConfig) *ResultWriter {
	return &ResultWriter{
		config: config,
		results: &TestResults{
			StatusCodes: make(map[string]int64),
			Agents:      make([]AgentResult, 0),
		},
	}
}

func (rw *ResultWriter) WriteResults(results *TestResults) error {
	timestamp := time.Now().Format("20060102-150405")

	for _, format := range rw.config.Formats {
		filename := fmt.Sprintf("%s-%s.%s", rw.config.Filename, timestamp, strings.ToLower(format))
		filepath := filepath.Join(rw.config.Directory, filename)

		if err := rw.writeFormat(results, format, filepath); err != nil {
			return fmt.Errorf("failed to write %s format: %w", format, err)
		}

		LogInfo("Test results written to %s", filepath)
	}

	return nil
}

func (rw *ResultWriter) writeFormat(results *TestResults, format, filepath string) error {
	switch strings.ToLower(format) {
	case "json":
		return rw.writeJSON(results, filepath)
	case "csv":
		return rw.writeCSV(results, filepath)
	case "xml":
		return rw.writeXML(results, filepath)
	case "yaml":
		return rw.writeYAML(results, filepath)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func (rw *ResultWriter) writeJSON(results *TestResults, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

func (rw *ResultWriter) writeCSV(results *TestResults, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"agent_id", "region", "requests", "errors", "success_rate",
		"avg_latency_ms", "status_200", "status_400", "status_500", "other_status",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write agent data
	for _, agent := range results.Agents {
		successRate := float64(agent.Requests-agent.Errors) / float64(agent.Requests) * 100
		if agent.Requests == 0 {
			successRate = 0
		}

		record := []string{
			agent.AgentID,
			agent.Region,
			fmt.Sprintf("%d", agent.Requests),
			fmt.Sprintf("%d", agent.Errors),
			fmt.Sprintf("%.2f", successRate),
			fmt.Sprintf("%.2f", agent.AvgLatencyMs),
			fmt.Sprintf("%d", agent.StatusCodes["200"]),
			fmt.Sprintf("%d", agent.StatusCodes["400"]),
			fmt.Sprintf("%d", agent.StatusCodes["500"]),
			fmt.Sprintf("%d", getTotalOtherStatus(agent.StatusCodes)),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	// Write summary row
	summaryRecord := []string{
		"TOTAL",
		"ALL",
		fmt.Sprintf("%d", results.TotalRequests),
		fmt.Sprintf("%d", results.TotalErrors),
		fmt.Sprintf("%.2f", results.SuccessRate),
		fmt.Sprintf("%.2f", results.AvgLatencyMs),
		fmt.Sprintf("%d", results.StatusCodes["200"]),
		fmt.Sprintf("%d", results.StatusCodes["400"]),
		fmt.Sprintf("%d", results.StatusCodes["500"]),
		fmt.Sprintf("%d", getTotalOtherStatus(results.StatusCodes)),
	}
	return writer.Write(summaryRecord)
}

func (rw *ResultWriter) writeXML(results *TestResults, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	return encoder.Encode(results)
}

func (rw *ResultWriter) writeYAML(results *TestResults, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()
	return encoder.Encode(results)
}

func getTotalOtherStatus(statusCodes map[string]int64) int64 {
	total := int64(0)
	for code, count := range statusCodes {
		if code != "200" && code != "400" && code != "500" {
			total += count
		}
	}
	return total
}

// Helper function to create test results from collected metrics
func CreateTestResults(testName string, startTime time.Time, testPlan *TestPlan, agents []AgentResult) *TestResults {
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	var totalRequests, totalErrors int64
	var totalLatency float64
	var minLatency, maxLatency float64 = 999999, 0
	statusCodes := make(map[string]int64)

	for _, agent := range agents {
		totalRequests += agent.Requests
		totalErrors += agent.Errors
		totalLatency += agent.AvgLatencyMs * float64(agent.Requests)

		if agent.AvgLatencyMs < minLatency && agent.AvgLatencyMs > 0 {
			minLatency = agent.AvgLatencyMs
		}
		if agent.AvgLatencyMs > maxLatency {
			maxLatency = agent.AvgLatencyMs
		}

		for code, count := range agent.StatusCodes {
			statusCodes[code] += count
		}
	}

	var avgLatency float64
	if totalRequests > 0 {
		avgLatency = totalLatency / float64(totalRequests)
	}

	successRate := float64(totalRequests-totalErrors) / float64(totalRequests) * 100
	if totalRequests == 0 {
		successRate = 0
	}

	requestsPerSec := float64(totalRequests) / duration.Seconds()

	return &TestResults{
		TestName:       testName,
		StartTime:      startTime,
		EndTime:        endTime,
		Duration:       duration.String(),
		TotalRequests:  totalRequests,
		TotalErrors:    totalErrors,
		SuccessRate:    successRate,
		AvgLatencyMs:   avgLatency,
		MinLatencyMs:   minLatency,
		MaxLatencyMs:   maxLatency,
		RequestsPerSec: requestsPerSec,
		StatusCodes:    statusCodes,
		Agents:         agents,
		Summary: TestSummary{
			ConfigUsed:        *testPlan,
			ActualConcurrency: len(agents),
			AgentCount:        len(agents),
		},
	}
}
