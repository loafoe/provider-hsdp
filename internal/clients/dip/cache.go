/*
Copyright 2025 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dip

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
)

// ClientCache reuses authenticated DIP clients across reconciles so the
// underlying go-dip-api iam.Client can serve its cached, TTL-refreshed
// access token instead of every Connect() call performing a fresh
// service-identity login. Entries are keyed by ProviderConfig identity and
// invalidated automatically if the resolved credentials change (e.g. secret
// rotation).
type ClientCache struct {
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	client     *Client
	configHash string
}

// NewClientCache creates an empty ClientCache.
func NewClientCache() *ClientCache {
	return &ClientCache{entries: make(map[string]*cacheEntry)}
}

// Cache is the process-wide client cache shared by every controller's
// connector, since they all resolve and authenticate against DIP the same
// way.
var Cache = NewClientCache()

// Get returns a cached client for key if one exists and cfg still matches the
// credentials it was created with, creating and caching a new client
// otherwise.
func (c *ClientCache) Get(key string, cfg Config) (*Client, error) {
	hash := configHash(cfg)

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[key]; ok && entry.configHash == hash {
		return entry.client, nil
	}

	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}

	log.Printf("dip: creating new client for provider config %q (cache miss)", key)
	c.entries[key] = &cacheEntry{client: client, configHash: hash}

	return client, nil
}

func configHash(cfg Config) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s",
		cfg.Region, cfg.Environment, cfg.ServiceID, cfg.ServicePrivateKey, cfg.TokenAudience)))
	return hex.EncodeToString(sum[:])
}
