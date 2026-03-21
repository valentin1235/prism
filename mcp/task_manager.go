package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// TaskStatus represents the lifecycle state of an analysis task.
type TaskStatus string

const (
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// IsTerminal returns true if the status represents a final state.
func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusCompleted || s == TaskStatusFailed
}

// StageStatus represents the lifecycle state of an individual pipeline stage.
type StageStatus string

const (
	StageStatusPending   StageStatus = "pending"
	StageStatusRunning   StageStatus = "running"
	StageStatusCompleted StageStatus = "completed"
	StageStatusFailed    StageStatus = "failed"
	StageStatusSkipped   StageStatus = "skipped"
)

// IsTerminal returns true if the stage status represents a final state.
func (s StageStatus) IsTerminal() bool {
	return s == StageStatusCompleted || s == StageStatusFailed || s == StageStatusSkipped
}

// StageName identifies the four stages of the analysis pipeline.
type StageName string

const (
	StageScope      StageName = "scope"
	StageSpecialist StageName = "specialist"
	StageInterview  StageName = "interview"
	StageSynthesis  StageName = "synthesis"
)

// AllStages returns the ordered pipeline stages.
func AllStages() []StageName {
	return []StageName{StageScope, StageSpecialist, StageInterview, StageSynthesis}
}

// StageProgress tracks progress within a single pipeline stage.
type StageProgress struct {
	Name      StageName   `json:"name"`
	Status    StageStatus `json:"status"`
	StartedAt *time.Time  `json:"started_at,omitempty"`
	EndedAt   *time.Time  `json:"ended_at,omitempty"`
	Detail    string      `json:"detail,omitempty"`
	// For parallel stages (specialist/interview): tracks sub-tasks
	Total     int `json:"total,omitempty"`
	Completed int `json:"completed,omitempty"`
	Failed    int `json:"failed,omitempty"`
}

// AnalysisTask represents a single analysis run managed by the MCP server.
type AnalysisTask struct {
	mu sync.RWMutex

	ID        string     `json:"id"`
	Status    TaskStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Configuration
	ContextID string `json:"context_id"`
	Model     string `json:"model"`
	StateDir  string `json:"state_dir"`
	ReportDir string `json:"report_dir"`

	// Pipeline progress (indexed by stage name for O(1) lookup)
	Stages map[StageName]*StageProgress `json:"stages"`

	// Result/error
	ReportPath string `json:"report_path,omitempty"`
	Error      string `json:"error,omitempty"`
}

// newAnalysisTask creates a new task with all stages initialized to pending.
func newAnalysisTask(contextID, model, stateDir, reportDir string) *AnalysisTask {
	now := time.Now().UTC()
	stages := make(map[StageName]*StageProgress, 4)
	for _, name := range AllStages() {
		stages[name] = &StageProgress{
			Name:   name,
			Status: StageStatusPending,
		}
	}
	return &AnalysisTask{
		ID:        generateTaskID(),
		Status:    TaskStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
		ContextID: contextID,
		Model:     model,
		StateDir:  stateDir,
		ReportDir: reportDir,
		Stages:    stages,
	}
}

// SetStatus transitions the task to a new status.
func (t *AnalysisTask) SetStatus(status TaskStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
	t.UpdatedAt = time.Now().UTC()
}

// SetError marks the task as failed with an error message.
func (t *AnalysisTask) SetError(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = TaskStatusFailed
	t.Error = err
	t.UpdatedAt = time.Now().UTC()
}

// SetReportPath marks the task as completed with a report path.
func (t *AnalysisTask) SetReportPath(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = TaskStatusCompleted
	t.ReportPath = path
	t.UpdatedAt = time.Now().UTC()
}

// StartStage transitions a stage to running.
func (t *AnalysisTask) StartStage(name StageName, detail string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.Stages[name]; ok {
		now := time.Now().UTC()
		s.Status = StageStatusRunning
		s.StartedAt = &now
		s.Detail = detail
	}
	t.UpdatedAt = time.Now().UTC()
}

// CompleteStage transitions a stage to completed.
func (t *AnalysisTask) CompleteStage(name StageName, detail string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.Stages[name]; ok {
		now := time.Now().UTC()
		s.Status = StageStatusCompleted
		s.EndedAt = &now
		if detail != "" {
			s.Detail = detail
		}
	}
	t.UpdatedAt = time.Now().UTC()
}

// FailStage transitions a stage to failed.
func (t *AnalysisTask) FailStage(name StageName, detail string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.Stages[name]; ok {
		now := time.Now().UTC()
		s.Status = StageStatusFailed
		s.EndedAt = &now
		s.Detail = detail
	}
	t.UpdatedAt = time.Now().UTC()
}

// SetStageParallel sets the total count for parallel sub-tasks in a stage.
func (t *AnalysisTask) SetStageParallel(name StageName, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.Stages[name]; ok {
		s.Total = total
		s.Completed = 0
		s.Failed = 0
	}
	t.UpdatedAt = time.Now().UTC()
}

// IncrStageCompleted increments the completed count for a parallel stage.
func (t *AnalysisTask) IncrStageCompleted(name StageName) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.Stages[name]; ok {
		s.Completed++
	}
	t.UpdatedAt = time.Now().UTC()
}

// IncrStageFailed increments the failed count for a parallel stage.
func (t *AnalysisTask) IncrStageFailed(name StageName) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.Stages[name]; ok {
		s.Failed++
	}
	t.UpdatedAt = time.Now().UTC()
}

// UpdateStageDetail updates only the detail text of a running stage
// without changing its status or timestamps. Used for sub-step progress
// within a single stage (e.g., "running seed analysis" → "running DA review").
func (t *AnalysisTask) UpdateStageDetail(name StageName, detail string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.Stages[name]; ok {
		s.Detail = detail
	}
	t.UpdatedAt = time.Now().UTC()
}

// UpdateDirs sets the context ID, state directory, and report directory.
// Used after task creation when directories are derived from the generated task ID.
func (t *AnalysisTask) UpdateDirs(contextID, stateDir, reportDir string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ContextID = contextID
	t.StateDir = stateDir
	t.ReportDir = reportDir
	t.UpdatedAt = time.Now().UTC()
}

// Snapshot returns a read-only copy of the task for safe external access.
// All pointer fields are deep-copied to ensure true immutability.
func (t *AnalysisTask) Snapshot() TaskSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stages := make([]StageProgress, 0, len(t.Stages))
	for _, name := range AllStages() {
		if s, ok := t.Stages[name]; ok {
			cp := *s
			// Deep-copy time pointers to prevent shared mutable state
			if s.StartedAt != nil {
				ts := *s.StartedAt
				cp.StartedAt = &ts
			}
			if s.EndedAt != nil {
				ts := *s.EndedAt
				cp.EndedAt = &ts
			}
			stages = append(stages, cp)
		}
	}

	return TaskSnapshot{
		ID:         t.ID,
		Status:     t.Status,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
		ContextID:  t.ContextID,
		Stages:     stages,
		ReportPath: t.ReportPath,
		Error:      t.Error,
	}
}

// TaskSnapshot is an immutable point-in-time view of a task for API responses.
type TaskSnapshot struct {
	ID         string          `json:"id"`
	Status     TaskStatus      `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	ContextID  string          `json:"context_id"`
	Stages     []StageProgress `json:"stages"`
	ReportPath string          `json:"report_path,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// TaskStore is a thread-safe in-memory store for analysis tasks.
// Tasks are lost on MCP server restart (by design - no checkpoint/recovery).
type TaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*AnalysisTask
}

// NewTaskStore creates a new empty task store.
func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks: make(map[string]*AnalysisTask),
	}
}

// Create adds a new task to the store and returns it.
func (s *TaskStore) Create(contextID, model, stateDir, reportDir string) *AnalysisTask {
	task := newAnalysisTask(contextID, model, stateDir, reportDir)
	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()
	return task
}

// Get retrieves a task by ID, returning nil if not found.
func (s *TaskStore) Get(taskID string) *AnalysisTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[taskID]
}

// Snapshot retrieves an immutable snapshot of a task by ID.
// Returns the snapshot and true if found, zero value and false otherwise.
func (s *TaskStore) Snapshot(taskID string) (TaskSnapshot, bool) {
	s.mu.RLock()
	task := s.tasks[taskID]
	s.mu.RUnlock()
	if task == nil {
		return TaskSnapshot{}, false
	}
	return task.Snapshot(), true
}

// Remove deletes a task from the store. Used for cleanup on creation failure.
func (s *TaskStore) Remove(taskID string) {
	s.mu.Lock()
	delete(s.tasks, taskID)
	s.mu.Unlock()
}

// List returns snapshots of all tasks, most recent first.
func (s *TaskStore) List() []TaskSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := make([]TaskSnapshot, 0, len(s.tasks))
	for _, task := range s.tasks {
		snapshots = append(snapshots, task.Snapshot())
	}
	// Sort by created_at descending (most recent first)
	for i := 0; i < len(snapshots); i++ {
		for j := i + 1; j < len(snapshots); j++ {
			if snapshots[j].CreatedAt.After(snapshots[i].CreatedAt) {
				snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
			}
		}
	}
	return snapshots
}

// generateTaskID creates a unique task identifier: "analyze-{12 hex chars}"
func generateTaskID() string {
	b := make([]byte, 6) // 6 bytes = 12 hex chars
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("analyze-%012x", time.Now().UnixNano())
	}
	return "analyze-" + hex.EncodeToString(b)
}
