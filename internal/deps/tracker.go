package deps

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"doctrus/internal/workspace"
)

type Tracker struct {
	basePath string
}

type FileInfo struct {
	Path     string    `json:"path"`
	Hash     string    `json:"hash"`
	ModTime  time.Time `json:"mod_time"`
	Size     int64     `json:"size"`
}

type TaskState struct {
	TaskKey     string     `json:"task_key"`
	InputHashes []FileInfo `json:"input_hashes"`
	Outputs     []FileInfo `json:"outputs"`
	LastRun     time.Time  `json:"last_run"`
	Success     bool       `json:"success"`
}

func NewTracker(basePath string) *Tracker {
	if basePath == "" {
		basePath, _ = os.Getwd()
	}
	return &Tracker{
		basePath: basePath,
	}
}

func (t *Tracker) ShouldRunTask(execution *workspace.TaskExecution, previousState *TaskState) (bool, error) {
	if previousState == nil {
		return true, nil
	}

	if !previousState.Success {
		return true, nil
	}

	currentInputs, err := t.computeInputHashes(execution)
	if err != nil {
		return true, fmt.Errorf("failed to compute input hashes: %w", err)
	}

	if !t.inputsMatch(currentInputs, previousState.InputHashes) {
		return true, nil
	}

	if !t.outputsExist(execution, previousState.Outputs) {
		return true, nil
	}

	return false, nil
}

func (t *Tracker) ComputeTaskState(execution *workspace.TaskExecution, success bool) (*TaskState, error) {
	inputs, err := t.computeInputHashes(execution)
	if err != nil {
		return nil, fmt.Errorf("failed to compute input hashes: %w", err)
	}

	outputs, err := t.computeOutputHashes(execution)
	if err != nil {
		return nil, fmt.Errorf("failed to compute output hashes: %w", err)
	}

	taskKey := fmt.Sprintf("%s:%s", execution.WorkspaceName, execution.TaskName)

	return &TaskState{
		TaskKey:     taskKey,
		InputHashes: inputs,
		Outputs:     outputs,
		LastRun:     time.Now(),
		Success:     success,
	}, nil
}

func (t *Tracker) computeInputHashes(execution *workspace.TaskExecution) ([]FileInfo, error) {
	var fileInfos []FileInfo

	for _, pattern := range execution.Task.Inputs {
		matches, err := t.resolveGlobPattern(execution.AbsPath, pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve input pattern %s: %w", pattern, err)
		}

		for _, match := range matches {
			info, err := t.computeFileInfo(match)
			if err != nil {
				return nil, fmt.Errorf("failed to compute hash for %s: %w", match, err)
			}
			fileInfos = append(fileInfos, *info)
		}
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].Path < fileInfos[j].Path
	})

	return fileInfos, nil
}

func (t *Tracker) computeOutputHashes(execution *workspace.TaskExecution) ([]FileInfo, error) {
	var fileInfos []FileInfo

	for _, pattern := range execution.Task.Outputs {
		matches, err := t.resolveGlobPattern(execution.AbsPath, pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			info, err := t.computeFileInfo(match)
			if err != nil {
				continue
			}
			fileInfos = append(fileInfos, *info)
		}
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].Path < fileInfos[j].Path
	})

	return fileInfos, nil
}

func (t *Tracker) resolveGlobPattern(basePath, pattern string) ([]string, error) {
	// Handle absolute patterns
	if filepath.IsAbs(pattern) {
		return t.globFiles(pattern)
	}

	// Join with base path for relative patterns
	fullPattern := filepath.Join(basePath, pattern)
	return t.globFiles(fullPattern)
}

func (t *Tracker) globFiles(pattern string) ([]string, error) {
	// Use doublestar for advanced glob patterns including **/*
	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern %s: %w", pattern, err)
	}

	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue // Skip files that can't be stat'd
		}
		if info.Mode().IsRegular() {
			files = append(files, match)
		}
	}

	return files, nil
}

func (t *Tracker) computeFileInfo(filePath string) (*FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", filePath)
	}

	hash, err := t.computeFileHash(filePath)
	if err != nil {
		return nil, err
	}

	relPath, err := filepath.Rel(t.basePath, filePath)
	if err != nil {
		relPath = filePath
	}

	return &FileInfo{
		Path:    relPath,
		Hash:    hash,
		ModTime: stat.ModTime(),
		Size:    stat.Size(),
	}, nil
}

func (t *Tracker) computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func (t *Tracker) inputsMatch(current, previous []FileInfo) bool {
	if len(current) != len(previous) {
		return false
	}

	for i, curr := range current {
		prev := previous[i]
		if curr.Path != prev.Path || curr.Hash != prev.Hash {
			return false
		}
	}

	return true
}

func (t *Tracker) outputsExist(execution *workspace.TaskExecution, outputs []FileInfo) bool {
	for _, output := range outputs {
		fullPath := filepath.Join(t.basePath, output.Path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (t *Tracker) GetChangedInputs(execution *workspace.TaskExecution, previousState *TaskState) ([]string, error) {
	if previousState == nil {
		return []string{"no previous state"}, nil
	}

	current, err := t.computeInputHashes(execution)
	if err != nil {
		return nil, err
	}

	var changed []string

	prevMap := make(map[string]FileInfo)
	for _, prev := range previousState.InputHashes {
		prevMap[prev.Path] = prev
	}

	for _, curr := range current {
		if prev, exists := prevMap[curr.Path]; !exists {
			changed = append(changed, fmt.Sprintf("new file: %s", curr.Path))
		} else if prev.Hash != curr.Hash {
			changed = append(changed, fmt.Sprintf("modified: %s", curr.Path))
		}
	}

	for _, prev := range previousState.InputHashes {
		found := false
		for _, curr := range current {
			if curr.Path == prev.Path {
				found = true
				break
			}
		}
		if !found {
			changed = append(changed, fmt.Sprintf("deleted: %s", prev.Path))
		}
	}

	return changed, nil
}