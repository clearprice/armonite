package main

import (
	"fmt"
	"time"
)

// RampUpStrategy defines how agents should ramp up their load
type RampUpStrategy struct {
	Type     RampUpType  `json:"type" yaml:"type"`
	Duration string      `json:"duration" yaml:"duration"`
	Phases   []RampPhase `json:"phases,omitempty" yaml:"phases,omitempty"`
}

// RampUpType defines the strategy type
type RampUpType string

const (
	RampUpTypeImmediate RampUpType = "immediate" // All agents start at full concurrency immediately
	RampUpTypeLinear    RampUpType = "linear"    // Linear increase over duration
	RampUpTypeStep      RampUpType = "step"      // Step-wise increase in phases
	RampUpTypeCustom    RampUpType = "custom"    // Custom phases with specific timing
)

// RampPhase defines a single phase in a ramp-up strategy
type RampPhase struct {
	Duration    string `json:"duration" yaml:"duration"`       // How long this phase lasts
	Concurrency int    `json:"concurrency" yaml:"concurrency"` // Target concurrency for this phase
	Mode        string `json:"mode" yaml:"mode"`               // "parallel" or "sequential"
}

// RampUpExecution contains runtime state for ramp-up execution
type RampUpExecution struct {
	Strategy          RampUpStrategy
	StartTime         time.Time
	CurrentPhase      int
	TargetConcurrency int
	MaxConcurrency    int
}

// RampUpCalculator handles ramp-up calculations and phase transitions
type RampUpCalculator struct {
	strategy       RampUpStrategy
	maxConcurrency int
	startTime      time.Time
	duration       time.Duration
}

// NewRampUpCalculator creates a new ramp-up calculator
func NewRampUpCalculator(strategy RampUpStrategy, maxConcurrency int) (*RampUpCalculator, error) {
	duration, err := time.ParseDuration(strategy.Duration)
	if err != nil {
		return nil, fmt.Errorf("invalid ramp-up duration: %w", err)
	}

	return &RampUpCalculator{
		strategy:       strategy,
		maxConcurrency: maxConcurrency,
		duration:       duration,
	}, nil
}

// Start begins the ramp-up execution
func (calc *RampUpCalculator) Start() *RampUpExecution {
	calc.startTime = time.Now()

	execution := &RampUpExecution{
		Strategy:          calc.strategy,
		StartTime:         calc.startTime,
		CurrentPhase:      0,
		TargetConcurrency: calc.getInitialConcurrency(),
		MaxConcurrency:    calc.maxConcurrency,
	}

	return execution
}

// GetCurrentConcurrency calculates the current target concurrency based on elapsed time
func (calc *RampUpCalculator) GetCurrentConcurrency(execution *RampUpExecution) int {
	elapsed := time.Since(execution.StartTime)

	switch calc.strategy.Type {
	case RampUpTypeImmediate:
		return calc.maxConcurrency

	case RampUpTypeLinear:
		if elapsed >= calc.duration {
			return calc.maxConcurrency
		}
		progress := float64(elapsed) / float64(calc.duration)
		return int(float64(calc.maxConcurrency) * progress)

	case RampUpTypeStep, RampUpTypeCustom:
		return calc.getPhaseBasedConcurrency(execution, elapsed)

	default:
		return calc.maxConcurrency
	}
}

// getPhaseBasedConcurrency calculates concurrency for phase-based strategies
func (calc *RampUpCalculator) getPhaseBasedConcurrency(execution *RampUpExecution, elapsed time.Duration) int {
	if len(calc.strategy.Phases) == 0 {
		return calc.maxConcurrency
	}

	var cumulativeDuration time.Duration

	for i, phase := range calc.strategy.Phases {
		phaseDuration, err := time.ParseDuration(phase.Duration)
		if err != nil {
			continue
		}

		if elapsed <= cumulativeDuration+phaseDuration {
			execution.CurrentPhase = i
			execution.TargetConcurrency = phase.Concurrency
			return phase.Concurrency
		}

		cumulativeDuration += phaseDuration
	}

	// If we've passed all phases, use max concurrency
	return calc.maxConcurrency
}

// getInitialConcurrency returns the starting concurrency
func (calc *RampUpCalculator) getInitialConcurrency() int {
	switch calc.strategy.Type {
	case RampUpTypeImmediate:
		return calc.maxConcurrency
	case RampUpTypeLinear:
		return 1
	case RampUpTypeStep, RampUpTypeCustom:
		if len(calc.strategy.Phases) > 0 {
			return calc.strategy.Phases[0].Concurrency
		}
		return 1
	default:
		return calc.maxConcurrency
	}
}

// IsComplete checks if the ramp-up is finished
func (calc *RampUpCalculator) IsComplete(execution *RampUpExecution) bool {
	elapsed := time.Since(execution.StartTime)
	return elapsed >= calc.duration
}

// GetRemainingDuration returns how much time is left in the ramp-up
func (calc *RampUpCalculator) GetRemainingDuration(execution *RampUpExecution) time.Duration {
	elapsed := time.Since(execution.StartTime)
	if elapsed >= calc.duration {
		return 0
	}
	return calc.duration - elapsed
}

// CreateDefaultRampUp creates a default immediate ramp-up strategy
func CreateDefaultRampUp() RampUpStrategy {
	return RampUpStrategy{
		Type:     RampUpTypeImmediate,
		Duration: "0s",
		Phases:   nil,
	}
}

// CreateLinearRampUp creates a linear ramp-up strategy
func CreateLinearRampUp(duration string) RampUpStrategy {
	return RampUpStrategy{
		Type:     RampUpTypeLinear,
		Duration: duration,
		Phases:   nil,
	}
}

// CreateStepRampUp creates a step-based ramp-up strategy
func CreateStepRampUp(duration string, steps int) RampUpStrategy {
	if steps <= 0 {
		steps = 3
	}

	phases := make([]RampPhase, steps)
	stepDuration := fmt.Sprintf("%.0fs", parseOrDefault(duration, 60*time.Second).Seconds()/float64(steps))

	for i := 0; i < steps; i++ {
		concurrency := (i + 1) * 100 / steps // Percentage-based
		phases[i] = RampPhase{
			Duration:    stepDuration,
			Concurrency: concurrency,
			Mode:        "parallel",
		}
	}

	return RampUpStrategy{
		Type:     RampUpTypeStep,
		Duration: duration,
		Phases:   phases,
	}
}

// CreateCustomRampUp creates a custom ramp-up strategy with specific phases
func CreateCustomRampUp(totalDuration string, phases []RampPhase) RampUpStrategy {
	return RampUpStrategy{
		Type:     RampUpTypeCustom,
		Duration: totalDuration,
		Phases:   phases,
	}
}

// Helper function to parse duration with fallback
func parseOrDefault(duration string, defaultDuration time.Duration) time.Duration {
	if d, err := time.ParseDuration(duration); err == nil {
		return d
	}
	return defaultDuration
}

// ValidateRampUpStrategy validates a ramp-up strategy configuration
func ValidateRampUpStrategy(strategy RampUpStrategy) error {
	// Validate duration
	if _, err := time.ParseDuration(strategy.Duration); err != nil {
		return fmt.Errorf("invalid ramp-up duration '%s': %w", strategy.Duration, err)
	}

	// Validate strategy type
	switch strategy.Type {
	case RampUpTypeImmediate, RampUpTypeLinear:
		// No additional validation needed

	case RampUpTypeStep, RampUpTypeCustom:
		if len(strategy.Phases) == 0 {
			return fmt.Errorf("phase-based ramp-up strategy must have at least one phase")
		}

		for i, phase := range strategy.Phases {
			if _, err := time.ParseDuration(phase.Duration); err != nil {
				return fmt.Errorf("invalid duration for phase %d: %w", i, err)
			}

			if phase.Concurrency < 0 {
				return fmt.Errorf("phase %d concurrency must be non-negative", i)
			}

			if phase.Mode != "parallel" && phase.Mode != "sequential" {
				return fmt.Errorf("phase %d mode must be 'parallel' or 'sequential'", i)
			}
		}

	default:
		return fmt.Errorf("unknown ramp-up strategy type: %s", strategy.Type)
	}

	return nil
}
