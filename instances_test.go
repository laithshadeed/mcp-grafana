package mcpgrafana

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestLoadInstancesFromFile(t *testing.T) {
	cfg := map[string]InstanceConfig{
		"cde":  {URL: "https://grafana.cde.example.com", ServiceAccountToken: "tok-cde"},
		"edge": {URL: "https://grafana.edge.example.com", ServiceAccountToken: "tok-edge"},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "instances.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := LoadInstancesFromFile(path)
	if err != nil {
		t.Fatalf("LoadInstancesFromFile: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(loaded))
	}
	if loaded["cde"].URL != "https://grafana.cde.example.com" {
		t.Errorf("unexpected cde URL: %s", loaded["cde"].URL)
	}
	if loaded["edge"].ServiceAccountToken != "tok-edge" {
		t.Errorf("unexpected edge token: %s", loaded["edge"].ServiceAccountToken)
	}
}

func TestLoadInstancesFromFileNotFound(t *testing.T) {
	_, err := LoadInstancesFromFile("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestInstanceStoreBasic(t *testing.T) {
	instances := map[string]InstanceConfig{
		"cde":  {URL: "https://grafana.cde.example.com", ServiceAccountToken: "tok-cde"},
		"edge": {URL: "https://grafana.edge.example.com", ServiceAccountToken: "tok-edge"},
	}
	store := &InstanceStore{
		instances: instances,
		clients:   make(map[string]*GrafanaClient),
	}

	if !store.HasInstances() {
		t.Error("HasInstances should be true")
	}

	names := store.InstanceNames()
	sort.Strings(names)
	want := []string{"cde", "edge"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Errorf("InstanceNames() = %v, want %v", names, want)
	}

	info := store.ListInstances()
	if len(info) != 2 {
		t.Errorf("ListInstances() returned %d items, want 2", len(info))
	}

	cfg := store.InstanceConfigByName("cde")
	if cfg == nil || cfg.URL != "https://grafana.cde.example.com" {
		t.Errorf("InstanceConfigByName(cde) = %v, want cde config", cfg)
	}

	cfg = store.InstanceConfigByName("nonexistent")
	if cfg != nil {
		t.Errorf("InstanceConfigByName(nonexistent) = %v, want nil", cfg)
	}
}

func TestInstanceStoreNil(t *testing.T) {
	var store *InstanceStore
	if store.HasInstances() {
		t.Error("nil store should not have instances")
	}
	if store.InstanceNames() != nil {
		t.Error("nil store should return nil names")
	}
	if store.ListInstances() != nil {
		t.Error("nil store should return nil list")
	}
	if store.InstanceConfigByName("x") != nil {
		t.Error("nil store should return nil config")
	}
	if _, err := store.GetClient(context.Background(), "x"); err == nil {
		t.Error("nil store GetClient should error")
	}
}

func TestInitInstanceStoreSkipsEmpty(t *testing.T) {
	orig := globalInstanceStore
	globalInstanceStore = nil
	defer func() { globalInstanceStore = orig }()

	InitInstanceStore(nil)
	if globalInstanceStore != nil {
		t.Error("InitInstanceStore(nil) should not set global store")
	}

	InitInstanceStore(map[string]InstanceConfig{})
	if globalInstanceStore != nil {
		t.Error("InitInstanceStore(empty) should not set global store")
	}
}
