package deps

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"doctrus/internal/config"
	"doctrus/internal/workspace"
)

func TestNewTracker(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
	}{
		{
			name:     "with base path",
			basePath: "/test/base",
		},
		{
			name:     "without base path",
			basePath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker(tt.basePath)
			if tracker == nil {
				t.Fatal("NewTracker() returned nil")
			}
			if tt.basePath != "" && tracker.basePath != tt.basePath {
				t.Errorf("Tracker basePath = %v, want %v", tracker.basePath, tt.basePath)
			}
			if tt.basePath == "" && tracker.basePath == "" {
				t.Error("Tracker basePath not set when empty string provided")
			}
		})
	}
}

func TestComputeFileInfo(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "test content"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	info, err := tracker.computeFileInfo(testFile)
	if err != nil {
		t.Fatalf("computeFileInfo() error = %v", err)
	}
	
	if info == nil {
		t.Fatal("computeFileInfo() returned nil")
	}
	
	if info.Path != "test.txt" {
		t.Errorf("FileInfo.Path = %v, want test.txt", info.Path)
	}
	
	if info.Hash == "" {
		t.Error("FileInfo.Hash is empty")
	}
	
	if info.Size != int64(len(testContent)) {
		t.Errorf("FileInfo.Size = %v, want %v", info.Size, len(testContent))
	}
}

func TestComputeFileInfoDirectory(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	
	_, err := tracker.computeFileInfo(subDir)
	if err == nil {
		t.Error("computeFileInfo() should error for directories")
	}
}

func TestComputeFileHash(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "hello world"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	hash1, err := tracker.computeFileHash(testFile)
	if err != nil {
		t.Fatalf("computeFileHash() error = %v", err)
	}
	
	if hash1 == "" {
		t.Error("computeFileHash() returned empty hash")
	}
	
	hash2, err := tracker.computeFileHash(testFile)
	if err != nil {
		t.Fatalf("computeFileHash() error on second call = %v", err)
	}
	
	if hash1 != hash2 {
		t.Error("computeFileHash() should return same hash for same file")
	}
	
	if err := os.WriteFile(testFile, []byte("different content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}
	
	hash3, err := tracker.computeFileHash(testFile)
	if err != nil {
		t.Fatalf("computeFileHash() error after modification = %v", err)
	}
	
	if hash1 == hash3 {
		t.Error("computeFileHash() should return different hash for modified file")
	}
}

func TestGlobFiles(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)
	
	testFiles := []string{
		filepath.Join(srcDir, "main.go"),
		filepath.Join(srcDir, "utils.go"),
		filepath.Join(srcDir, "test.js"),
		filepath.Join(tempDir, "README.md"),
	}
	
	for _, file := range testFiles {
		if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}
	
	tests := []struct {
		name     string
		pattern  string
		expected int
	}{
		{
			name:     "all go files",
			pattern:  filepath.Join(tempDir, "src", "*.go"),
			expected: 2,
		},
		{
			name:     "all files in src",
			pattern:  filepath.Join(tempDir, "src", "*"),
			expected: 3,
		},
		{
			name:     "specific file",
			pattern:  filepath.Join(tempDir, "README.md"),
			expected: 1,
		},
		{
			name:     "non-matching pattern",
			pattern:  filepath.Join(tempDir, "*.txt"),
			expected: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := tracker.globFiles(tt.pattern)
			if err != nil {
				t.Fatalf("globFiles() error = %v", err)
			}
			if len(matches) != tt.expected {
				t.Errorf("globFiles() returned %d matches, want %d", len(matches), tt.expected)
			}
		})
	}
}

func TestInputsMatch(t *testing.T) {
	tracker := NewTracker("/test")
	
	tests := []struct {
		name     string
		current  []FileInfo
		previous []FileInfo
		want     bool
	}{
		{
			name: "identical inputs",
			current: []FileInfo{
				{Path: "file1.txt", Hash: "hash1"},
				{Path: "file2.txt", Hash: "hash2"},
			},
			previous: []FileInfo{
				{Path: "file1.txt", Hash: "hash1"},
				{Path: "file2.txt", Hash: "hash2"},
			},
			want: true,
		},
		{
			name: "different hash",
			current: []FileInfo{
				{Path: "file1.txt", Hash: "hash1"},
			},
			previous: []FileInfo{
				{Path: "file1.txt", Hash: "hash2"},
			},
			want: false,
		},
		{
			name: "different path",
			current: []FileInfo{
				{Path: "file1.txt", Hash: "hash1"},
			},
			previous: []FileInfo{
				{Path: "file2.txt", Hash: "hash1"},
			},
			want: false,
		},
		{
			name: "different length",
			current: []FileInfo{
				{Path: "file1.txt", Hash: "hash1"},
			},
			previous: []FileInfo{
				{Path: "file1.txt", Hash: "hash1"},
				{Path: "file2.txt", Hash: "hash2"},
			},
			want: false,
		},
		{
			name:     "both empty",
			current:  []FileInfo{},
			previous: []FileInfo{},
			want:     true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.inputsMatch(tt.current, tt.previous)
			if result != tt.want {
				t.Errorf("inputsMatch() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestOutputsExist(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	existingFile := filepath.Join(tempDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	execution := &workspace.TaskExecution{
		WorkspaceName: "test",
		TaskName:      "build",
	}
	
	tests := []struct {
		name    string
		outputs []FileInfo
		want    bool
	}{
		{
			name: "all outputs exist",
			outputs: []FileInfo{
				{Path: "exists.txt"},
			},
			want: true,
		},
		{
			name: "output does not exist",
			outputs: []FileInfo{
				{Path: "missing.txt"},
			},
			want: false,
		},
		{
			name: "mixed existence",
			outputs: []FileInfo{
				{Path: "exists.txt"},
				{Path: "missing.txt"},
			},
			want: false,
		},
		{
			name:    "no outputs",
			outputs: []FileInfo{},
			want:    true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tracker.outputsExist(execution, tt.outputs)
			if result != tt.want {
				t.Errorf("outputsExist() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestShouldRunTask(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	execution := &workspace.TaskExecution{
		WorkspaceName: "test",
		TaskName:      "build",
		Task: &config.Task{
			Command: []string{"echo", "test"},
			Inputs:  []string{},
			Outputs: []string{},
		},
		AbsPath: tempDir,
	}
	
	tests := []struct {
		name          string
		previousState *TaskState
		want          bool
	}{
		{
			name:          "no previous state",
			previousState: nil,
			want:          true,
		},
		{
			name: "previous failure",
			previousState: &TaskState{
				Success: false,
			},
			want: true,
		},
		{
			name: "successful with matching inputs",
			previousState: &TaskState{
				Success:     true,
				InputHashes: []FileInfo{},
				Outputs:     []FileInfo{},
			},
			want: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tracker.ShouldRunTask(execution, tt.previousState)
			if err != nil {
				t.Fatalf("ShouldRunTask() error = %v", err)
			}
			if result != tt.want {
				t.Errorf("ShouldRunTask() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestComputeTaskState(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	inputFile := filepath.Join(tempDir, "input.txt")
	outputFile := filepath.Join(tempDir, "output.txt")
	
	if err := os.WriteFile(inputFile, []byte("input"), 0644); err != nil {
		t.Fatalf("Failed to create input file: %v", err)
	}
	if err := os.WriteFile(outputFile, []byte("output"), 0644); err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	
	execution := &workspace.TaskExecution{
		WorkspaceName: "test",
		TaskName:      "build",
		Task: &config.Task{
			Command: []string{"echo", "test"},
			Inputs:  []string{"input.txt"},
			Outputs: []string{"output.txt"},
		},
		AbsPath: tempDir,
	}
	
	state, err := tracker.ComputeTaskState(execution, true)
	if err != nil {
		t.Fatalf("ComputeTaskState() error = %v", err)
	}
	
	if state == nil {
		t.Fatal("ComputeTaskState() returned nil")
	}
	
	if state.TaskKey != "test:build" {
		t.Errorf("TaskState.TaskKey = %v, want test:build", state.TaskKey)
	}
	
	if !state.Success {
		t.Error("TaskState.Success should be true")
	}
	
	if len(state.InputHashes) != 1 {
		t.Errorf("TaskState.InputHashes length = %v, want 1", len(state.InputHashes))
	}
	
	if len(state.Outputs) != 1 {
		t.Errorf("TaskState.Outputs length = %v, want 1", len(state.Outputs))
	}
}

func TestGetChangedInputs(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}
	
	execution := &workspace.TaskExecution{
		WorkspaceName: "test",
		TaskName:      "build",
		Task: &config.Task{
			Inputs: []string{"file1.txt", "file2.txt"},
		},
		AbsPath: tempDir,
	}
	
	previousState := &TaskState{
		InputHashes: []FileInfo{
			{Path: "file1.txt", Hash: "oldhash1"},
			{Path: "file3.txt", Hash: "hash3"},
		},
	}
	
	changes, err := tracker.GetChangedInputs(execution, previousState)
	if err != nil {
		t.Fatalf("GetChangedInputs() error = %v", err)
	}
	
	if len(changes) == 0 {
		t.Error("GetChangedInputs() should detect changes")
	}
	
	hasModified := false
	hasNew := false
	hasDeleted := false
	
	for _, change := range changes {
		if contains(change, "modified") {
			hasModified = true
		}
		if contains(change, "new file") {
			hasNew = true
		}
		if contains(change, "deleted") {
			hasDeleted = true
		}
	}
	
	if !hasModified {
		t.Error("GetChangedInputs() should detect modified files")
	}
	if !hasNew {
		t.Error("GetChangedInputs() should detect new files")
	}
	if !hasDeleted {
		t.Error("GetChangedInputs() should detect deleted files")
	}
}

func TestGetChangedInputsNoPreviousState(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	execution := &workspace.TaskExecution{
		WorkspaceName: "test",
		TaskName:      "build",
		Task: &config.Task{
			Inputs: []string{},
		},
		AbsPath: tempDir,
	}
	
	changes, err := tracker.GetChangedInputs(execution, nil)
	if err != nil {
		t.Fatalf("GetChangedInputs() error = %v", err)
	}
	
	if len(changes) != 1 || changes[0] != "no previous state" {
		t.Errorf("GetChangedInputs() with nil state = %v, want [no previous state]", changes)
	}
}

func TestResolveGlobPattern(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)
	
	testFile := filepath.Join(srcDir, "test.go")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	tests := []struct {
		name     string
		basePath string
		pattern  string
		wantErr  bool
	}{
		{
			name:     "relative pattern",
			basePath: tempDir,
			pattern:  "src/*.go",
			wantErr:  false,
		},
		{
			name:     "absolute pattern",
			basePath: tempDir,
			pattern:  filepath.Join(tempDir, "src", "*.go"),
			wantErr:  false,
		},
		{
			name:     "invalid pattern",
			basePath: tempDir,
			pattern:  "[",
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches, err := tracker.resolveGlobPattern(tt.basePath, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveGlobPattern() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(matches) == 0 {
				t.Error("resolveGlobPattern() returned no matches")
			}
		})
	}
}

func TestComputeInputHashesWithPattern(t *testing.T) {
	tempDir := t.TempDir()
	tracker := NewTracker(tempDir)
	
	srcDir := filepath.Join(tempDir, "src")
	os.MkdirAll(srcDir, 0755)
	
	files := []string{
		filepath.Join(srcDir, "main.go"),
		filepath.Join(srcDir, "utils.go"),
		filepath.Join(srcDir, "test.js"),
	}
	
	for _, file := range files {
		if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", file, err)
		}
	}
	
	execution := &workspace.TaskExecution{
		Task: &config.Task{
			Inputs: []string{"src/*.go"},
		},
		AbsPath: tempDir,
	}
	
	hashes, err := tracker.computeInputHashes(execution)
	if err != nil {
		t.Fatalf("computeInputHashes() error = %v", err)
	}
	
	if len(hashes) != 2 {
		t.Errorf("computeInputHashes() returned %d hashes, want 2", len(hashes))
	}
	
	paths := []string{hashes[0].Path, hashes[1].Path}
	expectedPaths := []string{"src/main.go", "src/utils.go"}
	
	if !reflect.DeepEqual(paths, expectedPaths) {
		t.Errorf("computeInputHashes() paths = %v, want %v", paths, expectedPaths)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}