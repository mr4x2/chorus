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
	"sync"
	"time"

	"github.com/google/uuid"
)

// ConfigCache provides in-memory caching for runtime configurations
type ConfigCache struct {
	mu   sync.RWMutex
	data map[uuid.UUID]*cacheEntry
	ttl  time.Duration
}

type cacheEntry struct {
	config    *RuntimeConfig
	expiresAt time.Time
}

// NewConfigCache creates a new cache with TTL
func NewConfigCache(ttl time.Duration) *ConfigCache {
	cache := &ConfigCache{
		data: make(map[uuid.UUID]*cacheEntry),
		ttl:  ttl,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves a cached configuration
func (c *ConfigCache) Get(jobID uuid.UUID) *RuntimeConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.data[jobID]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.config
}

// Set stores a configuration in cache
func (c *ConfigCache) Set(jobID uuid.UUID, config *RuntimeConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[jobID] = &cacheEntry{
		config:    config,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Invalidate removes a specific entry from cache
func (c *ConfigCache) Invalidate(jobID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, jobID)
}

// InvalidateByProject removes all entries for a project
func (c *ConfigCache) InvalidateByProject(projectID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for jobID, entry := range c.data {
		if entry.config != nil && entry.config.ProjectID == projectID {
			delete(c.data, jobID)
		}
	}
}

// Clear removes all entries from cache
func (c *ConfigCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[uuid.UUID]*cacheEntry)
}

// Size returns the number of cached entries
func (c *ConfigCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.data)
}

// cleanup removes expired entries periodically
func (c *ConfigCache) cleanup() {
	ticker := time.NewTicker(c.ttl / 2) // Cleanup every half TTL
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for jobID, entry := range c.data {
			if now.After(entry.expiresAt) {
				delete(c.data, jobID)
			}
		}
		c.mu.Unlock()
	}
}
