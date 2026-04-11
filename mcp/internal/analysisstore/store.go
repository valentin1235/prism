package analysisstore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type AnalysisConfigRecord struct {
	TaskID               string
	Topic                string
	Model                string
	Adaptor              string
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

	_, err = db.Exec(`
		INSERT INTO analysis_tasks (
			task_id, topic, model, adaptor, context_id, state_dir, report_dir,
			input_context, ontology_scope, seed_hints, report_template, language, perspective_injection
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			topic = excluded.topic,
			model = excluded.model,
			adaptor = excluded.adaptor,
			context_id = excluded.context_id,
			state_dir = excluded.state_dir,
			report_dir = excluded.report_dir,
			input_context = excluded.input_context,
			ontology_scope = excluded.ontology_scope,
			seed_hints = excluded.seed_hints,
			report_template = excluded.report_template,
			language = excluded.language,
			perspective_injection = excluded.perspective_injection,
			updated_at = CURRENT_TIMESTAMP
	`, rec.TaskID, rec.Topic, rec.Model, rec.Adaptor, rec.ContextID, rec.StateDir, rec.ReportDir,
		rec.InputContext, rec.OntologyScope, rec.SeedHints, rec.ReportTemplate, rec.Language, rec.PerspectiveInjection)
	return err
}

func LoadAnalysisConfig(baseDir, taskID string) (AnalysisConfigRecord, bool, error) {
	var rec AnalysisConfigRecord
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
			task_id, topic, model, adaptor, context_id, state_dir, report_dir,
			COALESCE(input_context, ''),
			COALESCE(ontology_scope, ''),
			COALESCE(seed_hints, ''),
			COALESCE(report_template, ''),
			COALESCE(language, ''),
			COALESCE(perspective_injection, '')
		FROM analysis_tasks
		WHERE task_id = ?
	`, taskID).Scan(
		&rec.TaskID, &rec.Topic, &rec.Model, &rec.Adaptor, &rec.ContextID, &rec.StateDir, &rec.ReportDir,
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
	return rec, true, nil
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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (adaptor IN ('claude', 'codex'))
);
CREATE INDEX IF NOT EXISTS ix_analysis_tasks_state_dir ON analysis_tasks (state_dir);
`)
	return err
}
