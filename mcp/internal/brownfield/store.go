package brownfield

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// Repo represents a registered brownfield repository.
type Repo struct {
	RowID        int64  `json:"rowid"`
	Path         string `json:"path"`
	Name         string `json:"name"`
	Desc         string `json:"desc,omitempty"`
	IsDefault    bool   `json:"is_default"`
	RegisteredAt string `json:"registered_at"`
}

// Store manages brownfield repository persistence in SQLite.
type Store struct {
	mu sync.Mutex
	db *sql.DB
}

// NewStore opens (or creates) the brownfield SQLite database at ~/.prism/prism.db.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".prism")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create prism directory: %w", err)
	}
	dbPath := filepath.Join(dir, "prism.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)")
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}
	s := &Store{db: db}
	if err := s.initialize(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) initialize() error {
	schema := `
CREATE TABLE IF NOT EXISTS brownfield_repos (
    path TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    desc TEXT,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS ix_brownfield_repos_is_default ON brownfield_repos (is_default);
`
	_, err := s.db.Exec(schema)
	return err
}

// Register inserts or updates a single repository.
func (s *Store) Register(path, name, desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO brownfield_repos (path, name, desc) VALUES (?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name = COALESCE(NULLIF(excluded.name, ''), brownfield_repos.name),
			desc = COALESCE(NULLIF(excluded.desc, ''), brownfield_repos.desc)
	`, path, name, desc)
	return err
}

// BulkRegister inserts repos from a scan. Preserves existing is_default and desc.
func (s *Store) BulkRegister(repos []Repo) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO brownfield_repos (path, name) VALUES (?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name = COALESCE(NULLIF(excluded.name, ''), brownfield_repos.name)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	var errs []string
	for _, r := range repos {
		if _, err := stmt.Exec(r.Path, r.Name); err != nil {
			errs = append(errs, fmt.Sprintf("register %s: %v", r.Path, err))
			continue
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if len(errs) > 0 {
		return count, fmt.Errorf("%d rows failed: %s", len(errs), strings.Join(errs, "; "))
	}
	return count, nil
}

// List returns repos with pagination. If defaultOnly is true, returns only defaults.
func (s *Store) List(offset, limit int, defaultOnly bool) ([]Repo, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	where := ""
	if defaultOnly {
		where = "WHERE is_default = 1"
	}

	var total int
	err := s.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM brownfield_repos %s", where)).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	q := fmt.Sprintf(`
		SELECT rowid, path, name, COALESCE(desc, ''), is_default, registered_at
		FROM brownfield_repos %s
		ORDER BY name ASC
		LIMIT ? OFFSET ?
	`, where)

	effectiveLimit := limit
	if effectiveLimit <= 0 {
		effectiveLimit = 10000
	}

	rows, err := s.db.Query(q, effectiveLimit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var repos []Repo
	for rows.Next() {
		var r Repo
		if err := rows.Scan(&r.RowID, &r.Path, &r.Name, &r.Desc, &r.IsDefault, &r.RegisteredAt); err != nil {
			return nil, 0, fmt.Errorf("scan row: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, total, rows.Err()
}

// UpdateDefault toggles the is_default flag for a repo by path.
func (s *Store) UpdateDefault(path string, isDefault bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec("UPDATE brownfield_repos SET is_default = ? WHERE path = ?", isDefault, path)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("repo not found: %s", path)
	}
	return nil
}

// SetDefaultsByRowIDs clears all defaults and sets the given rowids as default.
func (s *Store) SetDefaultsByRowIDs(ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Validate all IDs exist before mutating
	for _, id := range ids {
		var exists int
		if err := tx.QueryRow("SELECT 1 FROM brownfield_repos WHERE rowid = ?", id).Scan(&exists); err != nil {
			return fmt.Errorf("rowid %d does not exist", id)
		}
	}

	if _, err := tx.Exec("UPDATE brownfield_repos SET is_default = 0"); err != nil {
		return err
	}
	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		args := make([]interface{}, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			args[i] = id
		}
		q := fmt.Sprintf("UPDATE brownfield_repos SET is_default = 1 WHERE rowid IN (%s)", strings.Join(placeholders, ","))
		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpdateDesc updates the description for a repo by path.
func (s *Store) UpdateDesc(path, desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec("UPDATE brownfield_repos SET desc = ? WHERE path = ?", desc, path)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("repo not found: %s", path)
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
