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
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker(t *testing.T) {
	logger := zerolog.Nop()
	config := ResilienceConfig{
		MaxRetries:       2,
		RetryDelay:       10 * time.Millisecond,
		RetryMultiplier:  2.0,
		MaxRetryDelay:    100 * time.Millisecond,
		FailureThreshold: 3,
		RecoveryTimeout:  100 * time.Millisecond,
		HalfOpenMaxCalls: 2,
		OperationTimeout: 1 * time.Second,
	}

	cb := NewCircuitBreaker(config, logger)

	// Test successful operation
	err := cb.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})
	assert.NoError(t, err)

	// Test failing operation that should trigger circuit breaker
	failCount := 0
	for i := 0; i < 5; i++ {
		err := cb.Execute(context.Background(), func(ctx context.Context) error {
			failCount++
			return errors.New("connection refused")
		})
		if i < 3 {
			assert.Error(t, err)
		} else {
			// Circuit should be open after 3 failures
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "circuit breaker is open")
		}
	}

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Test half-open state
	successCount := 0
	for i := 0; i < 3; i++ {
		err := cb.Execute(context.Background(), func(ctx context.Context) error {
			successCount++
			return nil
		})
		if i < 2 {
			assert.NoError(t, err)
		} else {
			// Should be back to closed state after 2 successes
			assert.NoError(t, err)
		}
	}
}

func TestResilientDB_RetryLogic(t *testing.T) {
	logger := zerolog.Nop()
	config := ResilienceConfig{
		MaxRetries:       2,
		RetryDelay:       10 * time.Millisecond,
		RetryMultiplier:  2.0,
		MaxRetryDelay:    100 * time.Millisecond,
		FailureThreshold: 5,
		RecoveryTimeout:  30 * time.Second,
		HalfOpenMaxCalls: 3,
		OperationTimeout: 1 * time.Second,
	}

	// Mock DB (we can't easily test with real GORM without setup)
	rdb := &ResilientDB{
		config:  config,
		breaker: NewCircuitBreaker(config, logger),
		logger:  logger,
	}

	// Test retryable error
	attemptCount := 0
	err := rdb.retryOperation(context.Background(), func(ctx context.Context) error {
		attemptCount++
		if attemptCount < 3 {
			return errors.New("connection refused")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, attemptCount)

	// Test non-retryable error
	attemptCount = 0
	err = rdb.retryOperation(context.Background(), func(ctx context.Context) error {
		attemptCount++
		return errors.New("record not found")
	})
	assert.Error(t, err)
	assert.Equal(t, 1, attemptCount) // Should not retry

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = rdb.retryOperation(ctx, func(ctx context.Context) error {
		return errors.New("connection refused")
	})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestShouldRetry(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultResilienceConfig()
	rdb := &ResilientDB{
		config: config,
		logger: logger,
	}

	// Test retryable errors
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

	for _, errMsg := range retryableErrors {
		assert.True(t, rdb.shouldRetry(errors.New(errMsg)), "Should retry error: %s", errMsg)
	}

	// Test non-retryable errors
	nonRetryableErrors := []error{
		context.Canceled,
		context.DeadlineExceeded,
		errors.New("record not found"),
		errors.New("validation failed"),
		errors.New("permission denied"),
	}

	for _, err := range nonRetryableErrors {
		assert.False(t, rdb.shouldRetry(err), "Should not retry error: %v", err)
	}
}

func TestDefaultResilienceConfig(t *testing.T) {
	config := DefaultResilienceConfig()

	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.RetryDelay)
	assert.Equal(t, 2.0, config.RetryMultiplier)
	assert.Equal(t, 5*time.Second, config.MaxRetryDelay)
	assert.Equal(t, 5, config.FailureThreshold)
	assert.Equal(t, 30*time.Second, config.RecoveryTimeout)
	assert.Equal(t, 3, config.HalfOpenMaxCalls)
	assert.Equal(t, 10*time.Second, config.OperationTimeout)
}
