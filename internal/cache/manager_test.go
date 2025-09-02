package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"doctrus/internal/deps"
)

// createTestManager creates a cache manager with a temporary directory for testing
func createTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	return manager, tempDir
}

// createTestTaskState creates a sample task state for testing
func createTestTaskState(taskKey string, success bool) *deps.TaskState {
	return &deps.TaskState{
		TaskKey:     taskKey,
		InputHashes: []deps.FileInfo{{Path: "test.txt", Hash: "abc123"}},
		Outputs:     []deps.FileInfo{{Path: "output.txt", Hash: "def456"}},
		LastRun:     time.Now(),
		Success:     success,
	}
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name     string
		cacheDir string
		wantDir  string
	}{
		{
			name:     "with custom cache dir",
			cacheDir: "/tmp/test-cache",
			wantDir:  "/tmp/test-cache",
		},
		{
			name:     "with default cache dir",
			cacheDir: "",
			wantDir:  "", // Will be set to default in actual test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.cacheDir)
			if manager == nil {
				t.Fatal("NewManager() returned nil")
			}
			if tt.cacheDir != "" && manager.cacheDir != tt.cacheDir {
				t.Errorf("Manager cacheDir = %v, want %v", manager.cacheDir, tt.cacheDir)
			}
			if tt.cacheDir == "" && manager.cacheDir == "" {
				t.Error("Manager cacheDir not set when empty string provided")
			}
		})
	}
}

func TestManagerInitialize(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache", "nested", "dirs")

	manager := NewManager(cacheDir)
	err := manager.Initialize()

	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("Initialize() did not create cache directory")
	}
}

func TestManagerSetAndGet(t *testing.T) {
	manager, _ := createTestManager(t)
	taskState := createTestTaskState("frontend:build", true)

	err := manager.Set("frontend:build", taskState, 0)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	retrieved, err := manager.Get("frontend:build")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved == nil {
		t.Fatal("Get() returned nil TaskState")
	}

	if retrieved.TaskKey != taskState.TaskKey {
		t.Errorf("Retrieved TaskKey = %v, want %v", retrieved.TaskKey, taskState.TaskKey)
	}

	if len(retrieved.InputHashes) != len(taskState.InputHashes) {
		t.Errorf("Retrieved InputHashes length = %v, want %v", len(retrieved.InputHashes), len(taskState.InputHashes))
	}

	if retrieved.Success != taskState.Success {
		t.Errorf("Retrieved Success = %v, want %v", retrieved.Success, taskState.Success)
	}
}

func TestManagerGetNonExistent(t *testing.T) {
	manager, _ := createTestManager(t)

	state, err := manager.Get("non:existent")
	if err != nil {
		t.Fatalf("Get() error = %v for non-existent key", err)
	}

	if state != nil {
		t.Error("Get() should return nil for non-existent key")
	}
}

func TestManagerSetWithTTL(t *testing.T) {
	manager, _ := createTestManager(t)
	taskState := createTestTaskState("backend:test", true)

	ttl := 100 * time.Millisecond
	err := manager.Set("backend:test", taskState, ttl)
	if err != nil {
		t.Fatalf("Set() with TTL error = %v", err)
	}

	retrieved, err := manager.Get("backend:test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("Get() returned nil immediately after Set()")
	}

	time.Sleep(150 * time.Millisecond)

	retrieved, err = manager.Get("backend:test")
	if err != nil {
		t.Fatalf("Get() error after TTL = %v", err)
	}
	if retrieved != nil {
		t.Error("Get() should return nil after TTL expiration")
	}
}

func TestManagerDelete(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	taskState := &deps.TaskState{
		TaskKey: "app:build",
		Success: true,
	}

	err := manager.Set("app:build", taskState, 0)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	err = manager.Delete("app:build")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	retrieved, err := manager.Get("app:build")
	if err != nil {
		t.Fatalf("Get() error after Delete = %v", err)
	}
	if retrieved != nil {
		t.Error("Get() should return nil after Delete()")
	}
}

func TestManagerDeleteNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	err := manager.Delete("non:existent")
	if err != nil {
		t.Error("Delete() should not error for non-existent key")
	}
}

func TestManagerClear(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	taskStates := map[string]*deps.TaskState{
		"frontend:build": {TaskKey: "frontend:build", Success: true},
		"frontend:test":  {TaskKey: "frontend:test", Success: true},
		"backend:build":  {TaskKey: "backend:build", Success: true},
	}

	for key, state := range taskStates {
		if err := manager.Set(key, state, 0); err != nil {
			t.Fatalf("Set() error for %s: %v", key, err)
		}
	}

	err := manager.Clear()
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	for key := range taskStates {
		retrieved, err := manager.Get(key)
		if err != nil {
			t.Fatalf("Get() error after Clear for %s: %v", key, err)
		}
		if retrieved != nil {
			t.Errorf("Get() should return nil after Clear() for %s", key)
		}
	}
}

func TestManagerList(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	taskStates := map[string]*deps.TaskState{
		"frontend:build": {TaskKey: "frontend:build", Success: true},
		"backend:test":   {TaskKey: "backend:test", Success: false},
	}

	for key, state := range taskStates {
		if err := manager.Set(key, state, 0); err != nil {
			t.Fatalf("Set() error for %s: %v", key, err)
		}
	}

	entries, err := manager.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(entries) != len(taskStates) {
		t.Errorf("List() returned %d entries, want %d", len(entries), len(taskStates))
	}

	foundKeys := make(map[string]bool)
	for _, entry := range entries {
		foundKeys[entry.TaskKey] = true
	}

	for key := range taskStates {
		if !foundKeys[key] {
			t.Errorf("List() missing key %s", key)
		}
	}
}

func TestManagerGetStats(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	if err := manager.Set("task1", &deps.TaskState{TaskKey: "task1"}, 0); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if err := manager.Set("task2", &deps.TaskState{TaskKey: "task2"}, 100*time.Millisecond); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	stats, err := manager.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats["total_entries"].(int) != 2 {
		t.Errorf("GetStats() total_entries = %v, want 2", stats["total_entries"])
	}

	if stats["expired_entries"].(int) != 1 {
		t.Errorf("GetStats() expired_entries = %v, want 1", stats["expired_entries"])
	}

	if stats["cache_dir"].(string) != tempDir {
		t.Errorf("GetStats() cache_dir = %v, want %v", stats["cache_dir"], tempDir)
	}
}

func TestManagerCleanExpired(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	if err := manager.Set("permanent", &deps.TaskState{TaskKey: "permanent"}, 0); err != nil {
		t.Fatalf("Set() error: %v", err)
	}
	if err := manager.Set("temporary", &deps.TaskState{TaskKey: "temporary"}, 50*time.Millisecond); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	err := manager.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired() error = %v", err)
	}

	permanent, err := manager.Get("permanent")
	if err != nil {
		t.Fatalf("Get() error for permanent: %v", err)
	}
	if permanent == nil {
		t.Error("CleanExpired() should not remove non-expired entries")
	}

	temporary, err := manager.Get("temporary")
	if err != nil {
		t.Fatalf("Get() error for temporary: %v", err)
	}
	if temporary != nil {
		t.Error("CleanExpired() should remove expired entries")
	}
}

func TestManagerInvalidateWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	taskStates := map[string]*deps.TaskState{
		"frontend:build": {TaskKey: "frontend:build"},
		"frontend:test":  {TaskKey: "frontend:test"},
		"backend:build":  {TaskKey: "backend:build"},
		"backend:test":   {TaskKey: "backend:test"},
	}

	for key, state := range taskStates {
		if err := manager.Set(key, state, 0); err != nil {
			t.Fatalf("Set() error for %s: %v", key, err)
		}
	}

	err := manager.InvalidateWorkspace("frontend")
	if err != nil {
		t.Fatalf("InvalidateWorkspace() error = %v", err)
	}

	frontendBuild, _ := manager.Get("frontend:build")
	frontendTest, _ := manager.Get("frontend:test")
	backendBuild, _ := manager.Get("backend:build")
	backendTest, _ := manager.Get("backend:test")

	if frontendBuild != nil || frontendTest != nil {
		t.Error("InvalidateWorkspace() should remove all frontend tasks")
	}

	if backendBuild == nil || backendTest == nil {
		t.Error("InvalidateWorkspace() should not remove backend tasks")
	}
}

func TestGetCachePath(t *testing.T) {
	manager := &Manager{
		cacheDir: "/test/cache",
	}

	tests := []struct {
		name     string
		taskKey  string
		expected string
	}{
		{
			name:     "simple task key",
			taskKey:  "frontend:build",
			expected: "/test/cache/frontendbuild.json",
		},
		{
			name:     "task key with special chars",
			taskKey:  "app/test:build*all",
			expected: "/test/cache/apptestbuildall.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := manager.getCachePath(tt.taskKey)
			if path != tt.expected {
				t.Errorf("getCachePath(%s) = %v, want %v", tt.taskKey, path, tt.expected)
			}
		})
	}
}

func TestCacheEntryJSON(t *testing.T) {
	entry := CacheEntry{
		TaskKey: "test:task",
		State: &deps.TaskState{
			TaskKey: "test:task",
			Success: true,
			LastRun: time.Now(),
		},
		CreatedAt: time.Now(),
		TTL:       5 * time.Minute,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal CacheEntry: %v", err)
	}

	var decoded CacheEntry
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal CacheEntry: %v", err)
	}

	if decoded.TaskKey != entry.TaskKey {
		t.Errorf("Decoded TaskKey = %v, want %v", decoded.TaskKey, entry.TaskKey)
	}

	if decoded.TTL != entry.TTL {
		t.Errorf("Decoded TTL = %v, want %v", decoded.TTL, entry.TTL)
	}
}
