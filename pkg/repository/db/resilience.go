// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// ResilienceConfig configures retry and circuit breaker behavior
type ResilienceConfig struct {
	// Retry configuration
	MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay      time.Duration `json:"retry_delay" yaml:"retry_delay"`
	RetryMultiplier float64       `json:"retry_multiplier" yaml:"retry_multiplier"`
	MaxRetryDelay   time.Duration `json:"max_retry_delay" yaml:"max_retry_delay"`

	// Circuit breaker configuration
	FailureThreshold int           `json:"failure_threshold" yaml:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout" yaml:"recovery_timeout"`
	HalfOpenMaxCalls int           `json:"half_open_max_calls" yaml:"half_open_max_calls"`

	// Timeout configuration
	OperationTimeout time.Duration `json:"operation_timeout" yaml:"operation_timeout"`
}

// DefaultResilienceConfig returns sensible defaults for production use
func DefaultResilienceConfig() ResilienceConfig {
	return ResilienceConfig{
		MaxRetries:       3,
		RetryDelay:       100 * time.Millisecond,
		RetryMultiplier:  2.0,
		MaxRetryDelay:    5 * time.Second,
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		HalfOpenMaxCalls: 3,
		OperationTimeout: 10 * time.Second,
	}
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	config        ResilienceConfig
	state         CircuitBreakerState
	failures      int
	lastFailTime  time.Time
	halfOpenCalls int
	mutex         sync.RWMutex
	logger        zerolog.Logger
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config ResilienceConfig, logger zerolog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitBreakerClosed,
		logger: logger,
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func(context.Context) error) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// Check if circuit is open
	if cb.state == CircuitBreakerOpen {
		if time.Since(cb.lastFailTime) > cb.config.RecoveryTimeout {
			cb.state = CircuitBreakerHalfOpen
			cb.halfOpenCalls = 0
			cb.logger.Info().Msg("Circuit breaker transitioning to half-open state")
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}

	// Check half-open call limit
	if cb.state == CircuitBreakerHalfOpen && cb.halfOpenCalls >= cb.config.HalfOpenMaxCalls {
		return fmt.Errorf("circuit breaker half-open call limit exceeded")
	}

	// Execute operation
	err := operation(ctx)
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case CircuitBreakerHalfOpen:
		cb.state = CircuitBreakerOpen
		cb.logger.Warn().Msg("Circuit breaker transitioning to open state (half-open failure)")
	case CircuitBreakerClosed:
		if cb.failures >= cb.config.FailureThreshold {
			cb.state = CircuitBreakerOpen
			cb.logger.Warn().Int("failures", cb.failures).Msg("Circuit breaker transitioning to open state")
		}
	}
}

func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case CircuitBreakerHalfOpen:
		cb.halfOpenCalls++
		if cb.halfOpenCalls >= cb.config.HalfOpenMaxCalls {
			cb.state = CircuitBreakerClosed
			cb.failures = 0
			cb.logger.Info().Msg("Circuit breaker transitioning to closed state")
		}
	case CircuitBreakerClosed:
		cb.failures = 0
	}
}

// ResilientDB wraps GORM DB with resilience features
type ResilientDB struct {
	db      *gorm.DB
	config  ResilienceConfig
	breaker *CircuitBreaker
	logger  zerolog.Logger
}

// NewResilientDB creates a new resilient database wrapper
func NewResilientDB(db *gorm.DB, config ResilienceConfig, logger zerolog.Logger) *ResilientDB {
	return &ResilientDB{
		db:      db,
		config:  config,
		breaker: NewCircuitBreaker(config, logger),
		logger:  logger,
	}
}

// ExecuteWithRetry executes a database operation with retry logic
func (r *ResilientDB) ExecuteWithRetry(ctx context.Context, operation func(context.Context) error) error {
	// Add operation timeout
	opCtx, cancel := context.WithTimeout(ctx, r.config.OperationTimeout)
	defer cancel()

	// Execute with circuit breaker
	return r.breaker.Execute(opCtx, func(ctx context.Context) error {
		return r.retryOperation(ctx, operation)
	})
}

// retryOperation implements exponential backoff retry logic
func (r *ResilientDB) retryOperation(ctx context.Context, operation func(context.Context) error) error {
	var lastErr error
	delay := r.config.RetryDelay

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Check if context is cancelled before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := operation(ctx)
		if err == nil {
			if attempt > 0 {
				r.logger.Info().Int("attempt", attempt+1).Msg("Database operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Don't retry on certain errors
		if !r.shouldRetry(err) {
			r.logger.Debug().Err(err).Int("attempt", attempt+1).Msg("Database operation failed with non-retryable error")
			return err
		}

		if attempt < r.config.MaxRetries {
			r.logger.Warn().Err(err).Int("attempt", attempt+1).Dur("delay", delay).Msg("Database operation failed, retrying")
			delay = time.Duration(float64(delay) * r.config.RetryMultiplier)
			if delay > r.config.MaxRetryDelay {
				delay = r.config.MaxRetryDelay
			}
		}
	}

	r.logger.Error().Err(lastErr).Int("attempts", r.config.MaxRetries+1).Msg("Database operation failed after all retries")
	return fmt.Errorf("operation failed after %d attempts: %w", r.config.MaxRetries+1, lastErr)
}

// shouldRetry determines if an error should trigger a retry
func (r *ResilientDB) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry context errors
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Don't retry certain GORM errors
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}

	// Retry connection-related errors
	errStr := err.Error()
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"timeout",
		"temporary failure",
		"too many connections",
		"server closed the connection",
		"network is unreachable",
		"no route to host",
	}

	for _, retryableErr := range retryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}

	return false
}

// GetDB returns the underlying GORM DB instance
func (r *ResilientDB) GetDB() *gorm.DB {
	return r.db
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			indexOf(s, substr) >= 0)))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
