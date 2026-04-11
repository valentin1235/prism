package analysisstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	taskpkg "github.com/heechul/prism-mcp/internal/task"
	_ "modernc.org/sqlite"
)

type AnalysisConfigRecord struct {
	TaskID               string
	Topic                string
	Model                string
	Adaptor              string
	CreatedAt            time.Time
	ContextID            string
	StateDir             string
	ReportDir            string
	InputContext         string
	OntologyScope        string
	SeedHints            string
	ReportTemplate       string
	Language             string
	PerspectiveInjection string
}

func SaveAnalysisConfig(baseDir string, rec AnalysisConfigRecord) error {
	db, err := open(baseDir)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := initialize(db); err != nil {
		return err
	}

	stagesJSON, err := json.Marshal(defaultStageProgress())
	if err != nil {
		return fmt.Errorf("marshal default stages: %w", err)
	}

	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	createdAt := formatSQLiteTimestamp(rec.CreatedAt)

	result, err := db.Exec(`
		INSERT INTO analysis_tasks (
			task_id, topic, model, adaptor, context_id, state_dir, report_dir,
			input_context, ontology_scope, seed_hints, report_template, language, perspective_injection,
			status, report_path, error, poll_count, stages_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(task_id) DO UPDATE SET
			topic = excluded.topic,
			model = excluded.model,
			context_id = excluded.context_id,
			state_dir = excluded.state_dir,
			report_dir = excluded.report_dir,
			input_context = excluded.input_context,
			ontology_scope = excluded.ontology_scope,
			seed_hints = excluded.seed_hints,
			report_template = excluded.report_template,
			language = excluded.language,
			perspective_injection = excluded.perspective_injection,
			status = excluded.status,
			report_path = excluded.report_path,
			error = excluded.error,
			poll_count = excluded.poll_count,
			stages_json = excluded.stages_json,
			created_at = excluded.created_at,
			updated_at = CURRENT_TIMESTAMP
		WHERE analysis_tasks.adaptor = excluded.adaptor
	`, rec.TaskID, rec.Topic, rec.Model, rec.Adaptor, rec.ContextID, rec.StateDir, rec.ReportDir,
		rec.InputContext, rec.OntologyScope, rec.SeedHints, rec.ReportTemplate, rec.Language, rec.PerspectiveInjection,
		string(taskpkg.TaskStatusQueued), "", "", 0, string(stagesJSON), createdAt)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows > 0 {
		return nil
	}

	var existingAdaptor string
	queryErr := db.QueryRow(`SELECT adaptor FROM analysis_tasks WHERE task_id = ?`, rec.TaskID).Scan(&existingAdaptor)
	if queryErr == sql.ErrNoRows {
		return fmt.Errorf("analysis task %s was not persisted", rec.TaskID)
	}
	if queryErr != nil {
		return queryErr
	}
	if existingAdaptor != rec.Adaptor {
		return fmt.Errorf("analysis task adaptor is immutable: task_id=%s existing=%s requested=%s", rec.TaskID, existingAdaptor, rec.Adaptor)
	}
	return nil
}

func LoadAnalysisConfig(baseDir, taskID string) (AnalysisConfigRecord, bool, error) {
	var rec AnalysisConfigRecord
	var rawCreatedAt string
	db, err := openReadOnly(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return rec, false, nil
		}
		return rec, false, err
	}
	defer db.Close()

	err = db.QueryRow(`
		SELECT
			task_id, topic, model, adaptor,
			strftime('%Y-%m-%dT%H:%M:%fZ', created_at),
			context_id, state_dir, report_dir,
			COALESCE(input_context, ''),
			COALESCE(ontology_scope, ''),
			COALESCE(seed_hints, ''),
			COALESCE(report_template, ''),
			COALESCE(language, ''),
			COALESCE(perspective_injection, '')
		FROM analysis_tasks
		WHERE task_id = ?
	`, taskID).Scan(
		&rec.TaskID, &rec.Topic, &rec.Model, &rec.Adaptor, &rawCreatedAt, &rec.ContextID, &rec.StateDir, &rec.ReportDir,
		&rec.InputContext, &rec.OntologyScope, &rec.SeedHints, &rec.ReportTemplate, &rec.Language, &rec.PerspectiveInjection,
	)
	if err == sql.ErrNoRows {
		return rec, false, nil
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return rec, false, nil
		}
		return rec, false, err
	}
	rec.CreatedAt = parseSQLiteTimestamp(rawCreatedAt)
	return rec, true, nil
}

func SaveTaskSnapshot(baseDir string, snapshot taskpkg.TaskSnapshot, pollCount int) error {
	db, err := open(baseDir)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := initialize(db); err != nil {
		return err
	}

	stagesJSON, err := json.Marshal(snapshot.Stages)
	if err != nil {
		return fmt.Errorf("marshal stages: %w", err)
	}

	result, err := db.Exec(`
		UPDATE analysis_tasks
		SET
			status = ?,
			context_id = ?,
			report_path = ?,
			error = ?,
			poll_count = ?,
			stages_json = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
	`, string(snapshot.Status), snapshot.ContextID, snapshot.ReportPath, snapshot.Error, pollCount, string(stagesJSON), snapshot.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("analysis task not found: %s", snapshot.ID)
	}
	return nil
}

func LoadTaskSnapshot(baseDir, taskID string) (taskpkg.TaskSnapshot, int, bool, error) {
	var snapshot taskpkg.TaskSnapshot

	db, err := openReadOnly(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, 0, false, nil
		}
		return snapshot, 0, false, err
	}
	defer db.Close()

	var (
		status     string
		createdRaw string
		updatedRaw string
		stagesJSON string
		pollCount  int
	)

	err = db.QueryRow(`
		SELECT
			task_id,
			COALESCE(status, 'queued'),
			strftime('%Y-%m-%dT%H:%M:%fZ', created_at),
			strftime('%Y-%m-%dT%H:%M:%fZ', updated_at),
			context_id,
			COALESCE(report_path, ''),
			COALESCE(error, ''),
			COALESCE(stages_json, '[]'),
			COALESCE(poll_count, 0)
		FROM analysis_tasks
		WHERE task_id = ?
	`, taskID).Scan(
		&snapshot.ID,
		&status,
		&createdRaw,
		&updatedRaw,
		&snapshot.ContextID,
		&snapshot.ReportPath,
		&snapshot.Error,
		&stagesJSON,
		&pollCount,
	)
	if err == sql.ErrNoRows {
		return snapshot, 0, false, nil
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
			return snapshot, 0, false, nil
		}
		return snapshot, 0, false, err
	}

	snapshot.Status = taskpkg.TaskStatus(status)
	snapshot.CreatedAt = parseSQLiteTimestamp(createdRaw)
	snapshot.UpdatedAt = parseSQLiteTimestamp(updatedRaw)
	if err := json.Unmarshal([]byte(stagesJSON), &snapshot.Stages); err != nil {
		return taskpkg.TaskSnapshot{}, 0, false, fmt.Errorf("parse persisted stages: %w", err)
	}

	return snapshot, pollCount, true, nil
}

func IncrementTaskPollCount(baseDir, taskID string) (int, error) {
	db, err := open(baseDir)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if err := initialize(db); err != nil {
		return 0, err
	}

	result, err := db.Exec(`
		UPDATE analysis_tasks
		SET
			poll_count = poll_count + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE task_id = ?
	`, taskID)
	if err != nil {
		return 0, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if rows == 0 {
		return 0, fmt.Errorf("analysis task not found: %s", taskID)
	}

	var pollCount int
	if err := db.QueryRow(`SELECT poll_count FROM analysis_tasks WHERE task_id = ?`, taskID).Scan(&pollCount); err != nil {
		return 0, err
	}
	return pollCount, nil
}

func DeleteAnalysisTask(baseDir, taskID string) error {
	db, err := open(baseDir)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := initialize(db); err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM analysis_tasks WHERE task_id = ?`, taskID)
	return err
}

func open(baseDir string) (*sql.DB, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return nil, fmt.Errorf("analysis store base dir is required")
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create analysis store dir: %w", err)
	}
	dbPath := filepath.Join(baseDir, "prism.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("open analysis store: %w", err)
	}
	return db, nil
}

func openReadOnly(baseDir string) (*sql.DB, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return nil, fmt.Errorf("analysis store base dir is required")
	}
	dbPath := filepath.Join(baseDir, "prism.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("open analysis store: %w", err)
	}
	return db, nil
}

func initialize(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS analysis_tasks (
    task_id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    model TEXT NOT NULL,
    adaptor TEXT NOT NULL,
    context_id TEXT NOT NULL,
    state_dir TEXT NOT NULL,
    report_dir TEXT NOT NULL,
    input_context TEXT,
    ontology_scope TEXT,
    seed_hints TEXT,
    report_template TEXT,
    language TEXT,
    perspective_injection TEXT,
    status TEXT NOT NULL DEFAULT 'queued',
    report_path TEXT,
    error TEXT,
    poll_count INTEGER NOT NULL DEFAULT 0,
    stages_json TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (adaptor IN ('claude', 'codex'))
);
CREATE INDEX IF NOT EXISTS ix_analysis_tasks_state_dir ON analysis_tasks (state_dir);
`)
	if err != nil {
		return err
	}

	migrations := []struct {
		name string
		ddl  string
	}{
		{"status", "ALTER TABLE analysis_tasks ADD COLUMN status TEXT NOT NULL DEFAULT 'queued'"},
		{"report_path", "ALTER TABLE analysis_tasks ADD COLUMN report_path TEXT"},
		{"error", "ALTER TABLE analysis_tasks ADD COLUMN error TEXT"},
		{"poll_count", "ALTER TABLE analysis_tasks ADD COLUMN poll_count INTEGER NOT NULL DEFAULT 0"},
		{"stages_json", "ALTER TABLE analysis_tasks ADD COLUMN stages_json TEXT NOT NULL DEFAULT '[]'"},
	}
	for _, migration := range migrations {
		ok, err := hasColumn(db, "analysis_tasks", migration.name)
		if err != nil {
			return err
		}
		if ok {
			continue
		}
		if _, err := db.Exec(migration.ddl); err != nil {
			return fmt.Errorf("add analysis_tasks.%s: %w", migration.name, err)
		}
	}

	return nil
}

func hasColumn(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid          int
			name         string
			columnType   string
			notNull      int
			defaultValue sql.NullString
			pk           int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, rows.Err()
}

func defaultStageProgress() []taskpkg.StageProgress {
	stages := make([]taskpkg.StageProgress, 0, len(taskpkg.AllStages()))
	for _, name := range taskpkg.AllStages() {
		stages = append(stages, taskpkg.StageProgress{
			Name:   name,
			Status: taskpkg.StageStatusPending,
		})
	}
	return stages
}

func parseSQLiteTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.000",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts.UTC()
		}
	}
	return time.Time{}
}

func formatSQLiteTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}
