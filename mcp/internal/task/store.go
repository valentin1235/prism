package task

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// MaxPollIterations is the maximum number of prism_task_status polls allowed
// per task before the task is auto-cancelled. With a 30-second polling interval,
// 120 iterations gives a 60-minute maximum wait time.
const MaxPollIterations = 120

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

	// PollCount tracks the number of prism_task_status calls for this task.
	// After MaxPollIterations is exceeded, the task is auto-cancelled and failed.
	PollCount int `json:"-"`

	// Ctx is the cancellable context for the pipeline goroutine.
	// Cancel terminates all in-flight subprocess work.
	Ctx    context.Context    `json:"-"`
	Cancel context.CancelFunc `json:"-"`

	persistHook      func(TaskSnapshot, int) error
	persistErrHook   func(error)
	persistErrRaised bool
}

// NewAnalysisTask creates a new task with all stages initialized to pending.
// sessionID is optional — when non-empty, the task ID becomes "analyze-{sessionID}".
func NewAnalysisTask(contextID, model, stateDir, reportDir, sessionID string) *AnalysisTask {
	now := time.Now().UTC()
	stages := make(map[StageName]*StageProgress, 4)
	for _, name := range AllStages() {
		stages[name] = &StageProgress{
			Name:   name,
			Status: StageStatusPending,
		}
	}
	return &AnalysisTask{
		ID:        generateTaskID(sessionID),
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
	t.Status = status
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// SetError marks the task as failed with an error message.
func (t *AnalysisTask) SetError(err string) {
	t.mu.Lock()
	t.Status = TaskStatusFailed
	t.Error = err
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// SetReportPath marks the task as completed with a report path.
func (t *AnalysisTask) SetReportPath(path string) {
	t.mu.Lock()
	t.Status = TaskStatusCompleted
	t.ReportPath = path
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// StartStage transitions a stage to running.
func (t *AnalysisTask) StartStage(name StageName, detail string) {
	t.mu.Lock()
	if s, ok := t.Stages[name]; ok {
		now := time.Now().UTC()
		s.Status = StageStatusRunning
		s.StartedAt = &now
		s.Detail = detail
	}
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// CompleteStage transitions a stage to completed.
func (t *AnalysisTask) CompleteStage(name StageName, detail string) {
	t.mu.Lock()
	if s, ok := t.Stages[name]; ok {
		now := time.Now().UTC()
		s.Status = StageStatusCompleted
		s.EndedAt = &now
		if detail != "" {
			s.Detail = detail
		}
	}
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// FailStage transitions a stage to failed.
func (t *AnalysisTask) FailStage(name StageName, detail string) {
	t.mu.Lock()
	if s, ok := t.Stages[name]; ok {
		now := time.Now().UTC()
		s.Status = StageStatusFailed
		s.EndedAt = &now
		s.Detail = detail
	}
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// SetStageParallel sets the total count for parallel sub-tasks in a stage.
func (t *AnalysisTask) SetStageParallel(name StageName, total int) {
	t.mu.Lock()
	if s, ok := t.Stages[name]; ok {
		s.Total = total
		s.Completed = 0
		s.Failed = 0
	}
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// IncrStageCompleted increments the completed count for a parallel stage.
func (t *AnalysisTask) IncrStageCompleted(name StageName) {
	t.mu.Lock()
	if s, ok := t.Stages[name]; ok {
		s.Completed++
	}
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// IncrStageFailed increments the failed count for a parallel stage.
func (t *AnalysisTask) IncrStageFailed(name StageName) {
	t.mu.Lock()
	if s, ok := t.Stages[name]; ok {
		s.Failed++
	}
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// UpdateStageDetail updates only the detail text of a running stage
// without changing its status or timestamps. Used for sub-step progress
// within a single stage (e.g., "running seed analysis" → "running DA review").
func (t *AnalysisTask) UpdateStageDetail(name StageName, detail string) {
	t.mu.Lock()
	if s, ok := t.Stages[name]; ok {
		s.Detail = detail
	}
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// IncrPollCount atomically increments the poll counter and returns the new count.
// Used by handleTaskStatus to enforce MaxPollIterations.
func (t *AnalysisTask) IncrPollCount() int {
	t.mu.Lock()
	t.PollCount++
	pollCount := t.PollCount
	t.mu.Unlock()
	t.persistIfConfigured()
	return pollCount
}

// UpdateDirs sets the context ID, state directory, and report directory.
// Used after task creation when directories are derived from the generated task ID.
func (t *AnalysisTask) UpdateDirs(contextID, stateDir, reportDir string) {
	t.mu.Lock()
	t.ContextID = contextID
	t.StateDir = stateDir
	t.ReportDir = reportDir
	t.UpdatedAt = time.Now().UTC()
	t.mu.Unlock()
	t.persistIfConfigured()
}

// GetID returns the task ID under a read lock.
func (t *AnalysisTask) GetID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ID
}

// GetStateDir returns the task state directory under a read lock.
func (t *AnalysisTask) GetStateDir() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.StateDir
}

// GetReportDir returns the task report directory under a read lock.
func (t *AnalysisTask) GetReportDir() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ReportDir
}

// Snapshot returns a read-only copy of the task for safe external access.
// All pointer fields are deep-copied to ensure true immutability.
func (t *AnalysisTask) Snapshot() TaskSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.snapshotLocked()
}

func (t *AnalysisTask) SnapshotWithPollCount() (TaskSnapshot, int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.snapshotLocked(), t.PollCount
}

func (t *AnalysisTask) snapshotLocked() TaskSnapshot {
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

func (t *AnalysisTask) SetPersistenceHook(fn func(TaskSnapshot, int) error) error {
	t.mu.Lock()
	t.persistHook = fn
	t.persistErrRaised = false
	t.mu.Unlock()
	return t.persistIfConfigured()
}

func (t *AnalysisTask) SetPersistenceErrorHook(fn func(error)) {
	t.mu.Lock()
	t.persistErrHook = fn
	t.mu.Unlock()
}

func (t *AnalysisTask) DisablePersistence() {
	t.mu.Lock()
	t.persistHook = nil
	t.persistErrHook = nil
	t.persistErrRaised = true
	t.mu.Unlock()
}

func (t *AnalysisTask) persistIfConfigured() error {
	t.mu.RLock()
	fn := t.persistHook
	if fn == nil {
		t.mu.RUnlock()
		return nil
	}
	snapshot := t.snapshotLocked()
	pollCount := t.PollCount
	t.mu.RUnlock()
	if err := fn(snapshot, pollCount); err != nil {
		log.Printf("[%s] task snapshot persistence failed: %v", snapshot.ID, err)
		t.notifyPersistError(err)
		return err
	}
	return nil
}

func (t *AnalysisTask) notifyPersistError(err error) {
	t.mu.Lock()
	if t.persistErrRaised {
		t.mu.Unlock()
		return
	}
	t.persistErrRaised = true
	hook := t.persistErrHook
	t.mu.Unlock()

	if hook != nil {
		hook(err)
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

// TaskStore is a thread-safe in-memory store for active analysis tasks.
// Long-lived task snapshots may also be persisted elsewhere for restart-safe reads.
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
func (s *TaskStore) Create(contextID, model, stateDir, reportDir, sessionID string) *AnalysisTask {
	task := NewAnalysisTask(contextID, model, stateDir, reportDir, sessionID)
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
	// Sort by created_at descending (most recent first), break ties by ID descending
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].CreatedAt.Equal(snapshots[j].CreatedAt) {
			return snapshots[i].ID > snapshots[j].ID
		}
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})
	return snapshots
}

// generateTaskID creates a unique task identifier.
// If sessionID is non-empty, returns "analyze-{sessionID}" for deterministic tracking.
// Otherwise returns "analyze-{12 hex chars}" with a random suffix.
func generateTaskID(sessionID string) string {
	if sessionID != "" {
		return "analyze-" + sessionID
	}
	b := make([]byte, 6) // 6 bytes = 12 hex chars
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("analyze-%012x", time.Now().UnixNano())
	}
	return "analyze-" + hex.EncodeToString(b)
}
