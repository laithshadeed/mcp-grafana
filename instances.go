package mcpgrafana

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
)

const grafanaInstancesFileEnvVar = "GRAFANA_INSTANCES_FILE"

// InstanceConfig holds connection details for a single Grafana instance.
type InstanceConfig struct {
	URL                 string `json:"url"`
	ServiceAccountToken string `json:"service_account_token"`
}

// InstanceStore holds multiple Grafana instance configs and caches clients.
type InstanceStore struct {
	mu        sync.RWMutex
	instances map[string]InstanceConfig
	clients   map[string]*GrafanaClient
}

var globalInstanceStore *InstanceStore

// InitInstanceStore initializes the global instance store from the given map.
func InitInstanceStore(instances map[string]InstanceConfig) {
	if len(instances) == 0 {
		return
	}
	globalInstanceStore = &InstanceStore{
		instances: instances,
		clients:   make(map[string]*GrafanaClient),
	}
}

// GlobalInstanceStore returns the global instance store, or nil if not configured.
func GlobalInstanceStore() *InstanceStore {
	return globalInstanceStore
}

// ResetInstanceStore clears the global instance store (for tests).
func ResetInstanceStore() {
	globalInstanceStore = nil
}

// HasInstances returns true if any instances are configured.
func (s *InstanceStore) HasInstances() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.instances) > 0
}

// InstanceNames returns sorted list of available instance names.
func (s *InstanceStore) InstanceNames() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.instances))
	for name := range s.instances {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// InstanceInfo describes an instance for listing purposes (URL without token).
type InstanceInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ListInstances returns info about all configured instances.
func (s *InstanceStore) ListInstances() []InstanceInfo {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]InstanceInfo, 0, len(s.instances))
	for name, cfg := range s.instances {
		result = append(result, InstanceInfo{Name: name, URL: cfg.URL})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// InstanceConfigByName returns the config for the named instance, or nil.
func (s *InstanceStore) InstanceConfigByName(name string) *InstanceConfig {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cfg, ok := s.instances[name]; ok {
		return &cfg
	}
	return nil
}

// InstanceMap returns the underlying map (for test cleanup via re-init).
func (s *InstanceStore) InstanceMap() map[string]InstanceConfig {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]InstanceConfig, len(s.instances))
	for k, v := range s.instances {
		cp[k] = v
	}
	return cp
}

// GetClient returns a cached Grafana client for the given instance name,
// creating one if necessary.
func (s *InstanceStore) GetClient(ctx context.Context, name string) (*GrafanaClient, error) {
	if s == nil {
		return nil, fmt.Errorf("no instances configured")
	}

	// Fast path: check cache with read lock
	s.mu.RLock()
	if client, ok := s.clients[name]; ok {
		s.mu.RUnlock()
		return client, nil
	}
	s.mu.RUnlock()

	// Slow path: create client under write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := s.clients[name]; ok {
		return client, nil
	}

	inst, ok := s.instances[name]
	if !ok {
		return nil, fmt.Errorf("unknown endpoint %q; use list_endpoints to see available instances", name)
	}

	client := NewGrafanaClient(ctx, inst.URL, inst.ServiceAccountToken, nil)
	s.clients[name] = client
	return client, nil
}

// LoadInstancesFromFile reads instance configs from a JSON file.
func LoadInstancesFromFile(path string) (map[string]InstanceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read instances file %s: %w", path, err)
	}
	var instances map[string]InstanceConfig
	if err := json.Unmarshal(data, &instances); err != nil {
		return nil, fmt.Errorf("parse instances file %s: %w", path, err)
	}
	return instances, nil
}

// LoadInstancesFromEnv loads instances from the file specified by
// GRAFANA_INSTANCES_FILE. Returns nil if the env var is not set.
func LoadInstancesFromEnv() map[string]InstanceConfig {
	path := os.Getenv(grafanaInstancesFileEnvVar)
	if path == "" {
		return nil
	}
	instances, err := LoadInstancesFromFile(path)
	if err != nil {
		slog.Default().Error("Failed to load instances file", "path", path, "error", err)
		return nil
	}
	return instances
}
