package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"doctrus/internal/deps"
)

type Manager struct {
	cacheDir string
	basePath string
}

type CacheEntry struct {
	TaskKey   string           `json:"task_key"`
	State     *deps.TaskState  `json:"state"`
	CreatedAt time.Time        `json:"created_at"`
	TTL       time.Duration    `json:"ttl,omitempty"`
}

func NewManager(cacheDir string, basePath string) *Manager {
	if cacheDir == "" {
		// Check if running in Docker container with custom cache dir
		if envCacheDir := os.Getenv("DOCTRUS_CACHE_DIR"); envCacheDir != "" {
			cacheDir = envCacheDir
		} else if basePath != "" {
			// Use base path (where doctrus.yml is) for cache directory
			cacheDir = filepath.Join(basePath, ".doctrus", "cache")
		} else {
			// Fallback to current working directory
			cwd, _ := os.Getwd()
			cacheDir = filepath.Join(cwd, ".doctrus", "cache")
		}
	}
	return &Manager{
		cacheDir: cacheDir,
		basePath: basePath,
	}
}

func (m *Manager) Initialize() error {
	return os.MkdirAll(m.cacheDir, 0755)
}

func (m *Manager) Get(taskKey string) (*deps.TaskState, error) {
	cachePath := m.getCachePath(taskKey)
	
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to parse cache entry: %w", err)
	}

	if entry.TTL > 0 && time.Since(entry.CreatedAt) > entry.TTL {
		m.Delete(taskKey)
		return nil, nil
	}

	return entry.State, nil
}

func (m *Manager) Set(taskKey string, state *deps.TaskState, ttl time.Duration) error {
	if err := m.Initialize(); err != nil {
		return err
	}

	entry := CacheEntry{
		TaskKey:   taskKey,
		State:     state,
		CreatedAt: time.Now(),
		TTL:       ttl,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	cachePath := m.getCachePath(taskKey)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

func (m *Manager) Delete(taskKey string) error {
	cachePath := m.getCachePath(taskKey)
	err := os.Remove(cachePath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (m *Manager) Clear() error {
	if _, err := os.Stat(m.cacheDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		filePath := filepath.Join(m.cacheDir, entry.Name())
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove cache file %s: %w", filePath, err)
		}
	}

	return nil
}

func (m *Manager) List() ([]CacheEntry, error) {
	if _, err := os.Stat(m.cacheDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	var cacheEntries []CacheEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(m.cacheDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			continue
		}

		cacheEntries = append(cacheEntries, cacheEntry)
	}

	return cacheEntries, nil
}

func (m *Manager) GetStats() (map[string]interface{}, error) {
	entries, err := m.List()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_entries": len(entries),
		"cache_dir":     m.cacheDir,
	}

	if cacheInfo, err := os.Stat(m.cacheDir); err == nil {
		stats["cache_dir_size"] = cacheInfo.Size()
	}

	expired := 0
	for _, entry := range entries {
		if entry.TTL > 0 && time.Since(entry.CreatedAt) > entry.TTL {
			expired++
		}
	}
	stats["expired_entries"] = expired

	return stats, nil
}

func (m *Manager) CleanExpired() error {
	entries, err := m.List()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.TTL > 0 && time.Since(entry.CreatedAt) > entry.TTL {
			if err := m.Delete(entry.TaskKey); err != nil {
				return fmt.Errorf("failed to delete expired cache entry %s: %w", entry.TaskKey, err)
			}
		}
	}

	return nil
}

func (m *Manager) getCachePath(taskKey string) string {
	filename := fmt.Sprintf("%s.json", taskKey)
	for _, char := range []string{":", "/", "\\", "*", "?", "\"", "<", ">", "|"} {
		filename = strings.ReplaceAll(filename, char, "")
	}
	return filepath.Join(m.cacheDir, filename)
}

func (m *Manager) InvalidateWorkspace(workspaceName string) error {
	entries, err := m.List()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.TaskKey, workspaceName+":") {
			if err := m.Delete(entry.TaskKey); err != nil {
				return fmt.Errorf("failed to invalidate cache for %s: %w", entry.TaskKey, err)
			}
		}
	}

	return nil
}