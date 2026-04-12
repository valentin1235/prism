package brownfield

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	prismconfig "github.com/heechul/prism-mcp/internal/config"
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

// MCPServer represents a scanned MCP server snapshot row.
type MCPServer struct {
	Name         string `json:"name"`
	Path         string `json:"path,omitempty"`
	Desc         string `json:"desc"`
	IsDefault    bool   `json:"is_default"`
	RegisteredAt string `json:"registered_at"`
	Visible      bool   `json:"visible,omitempty"`
	VisibilityOK bool   `json:"visibility_ok,omitempty"`
	Command      string `json:"-"`
}

// MCPServerSnapshot represents one MCP server row from the latest scan snapshot.
type MCPServerSnapshot struct {
	Name         string  `json:"name"`
	Path         *string `json:"path,omitempty"`
	Desc         string  `json:"desc"`
	IsDefault    bool    `json:"is_default"`
	RegisteredAt string  `json:"registered_at"`
}

// Entry represents a unified brownfield entry (repo or MCP server).
type Entry struct {
	RowID        int64  `json:"rowid"`
	Type         string `json:"type"`           // "repo" or "mcp"
	Key          string `json:"key"`            // repo: path, mcp: name
	Name         string `json:"name"`
	Desc         string `json:"desc,omitempty"`
	Path         string `json:"path,omitempty"` // repo: same as key, mcp: optional
	IsDefault    bool   `json:"is_default"`
	RegisteredAt string `json:"registered_at"`
}

// Store manages brownfield repository persistence in SQLite.
type Store struct {
	mu sync.Mutex
	db *sql.DB
}

type sqliteQueryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// SQLiteTableColumn describes one column from SQLite PRAGMA table_info output.
type SQLiteTableColumn struct {
	Name         string
	Type         string
	NotNull      int
	DefaultValue sql.NullString
	PKOrdinal    int
}

// SQLiteTableSchema captures sqlite_master existence plus PRAGMA metadata for a table.
type SQLiteTableSchema struct {
	Name      string
	Exists    bool
	CreateSQL string
	Columns   []SQLiteTableColumn
}

// RuntimeSQLiteTableMetadata captures the active SQLite handle, resolved
// database path, and sqlite_master/PRAGMA-derived schema state for one table
// from the same runtime SQLite source.
type RuntimeSQLiteTableMetadata struct {
	DB           *sql.DB
	DatabasePath string
	Table        SQLiteTableSchema
}

const unresolvedMCPDescriptionFormat = "MCP server %s (tool metadata unavailable at scan time)"

const (
	entriesTableName           = "brownfield_entries"
	legacyRepoTableName        = "brownfield_repos"
	legacyMCPSnapshotTableName = "brownfield_mcps"
	mcpSnapshotTableName       = "mcp_server_snapshot"
)

// NewStore opens (or creates) the brownfield SQLite database at the shared
// Prism runtime SQLite path.
func NewStore() (*Store, error) {
	return NewStoreAt(prismconfig.RuntimeSQLitePath())
}

// OpenRuntimeSQLiteStore opens the migrated Prism runtime SQLite database in
// read-only mode for evaluator-facing inspection.
func OpenRuntimeSQLiteStore() (*Store, error) {
	return OpenStoreAt(prismconfig.RuntimeSQLitePath())
}

// NewStoreAt opens (or creates) the brownfield SQLite database at dbPath.
func NewStoreAt(dbPath string) (*Store, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("brownfield database path is required")
	}
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create prism directory: %w", err)
	}
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
	entriesSchema := `
CREATE TABLE IF NOT EXISTS brownfield_entries (
    type TEXT NOT NULL CHECK(type IN ('repo', 'mcp')),
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    desc TEXT,
    path TEXT,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(type, key)
);
CREATE INDEX IF NOT EXISTS ix_brownfield_entries_is_default ON brownfield_entries (is_default);
CREATE INDEX IF NOT EXISTS ix_brownfield_entries_type ON brownfield_entries (type);
`
	if _, err := s.db.Exec(entriesSchema); err != nil {
		return err
	}
	return s.migrateFromLegacyTables()
}

func (s *Store) migrateFromLegacyTables() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	migrated := false

	// Migrate from brownfield_repos if it exists
	if repoExists, err := sqliteTableExists(tx, legacyRepoTableName); err != nil {
		return err
	} else if repoExists {
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO brownfield_entries (type, key, name, desc, path, is_default, registered_at)
			SELECT 'repo', path, name, desc, path, is_default, registered_at
			FROM brownfield_repos
		`); err != nil {
			return fmt.Errorf("migrate repos: %w", err)
		}
		if _, err := tx.Exec(`DROP TABLE brownfield_repos`); err != nil {
			return fmt.Errorf("drop legacy repo table: %w", err)
		}
		migrated = true
	}

	// Migrate from mcp_server_snapshot if it exists
	if mcpExists, err := sqliteTableExists(tx, mcpSnapshotTableName); err != nil {
		return err
	} else if mcpExists {
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO brownfield_entries (type, key, name, desc, path, is_default, registered_at)
			SELECT 'mcp', name, name, desc, path, 0, registered_at
			FROM mcp_server_snapshot
		`); err != nil {
			return fmt.Errorf("migrate mcps: %w", err)
		}
		if _, err := tx.Exec(`DROP TABLE mcp_server_snapshot`); err != nil {
			return fmt.Errorf("drop legacy mcp table: %w", err)
		}
		migrated = true
	}

	// Also drop the legacy brownfield_mcps table if it exists
	if legacyExists, err := sqliteTableExists(tx, legacyMCPSnapshotTableName); err != nil {
		return err
	} else if legacyExists {
		if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE %s`, legacyMCPSnapshotTableName)); err != nil {
			return fmt.Errorf("drop legacy mcps table: %w", err)
		}
		migrated = true
	}

	if !migrated {
		return nil
	}
	return tx.Commit()
}

// SQLiteTableExists checks sqlite_master for the named table on the current
// Store connection.
func (s *Store) SQLiteTableExists(name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sqliteTableExists(s.db, name)
}

// SQLiteTableSchema inspects sqlite_master and PRAGMA table_info for the named
// table on the current Store connection.
func (s *Store) SQLiteTableSchema(name string) (SQLiteTableSchema, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sqliteTableSchema(s.db, name)
}

// RuntimeSQLiteTableMetadata returns the active database handle plus
// sqlite_master/PRAGMA-derived schema state for one table from the current
// Store connection.
func (s *Store) RuntimeSQLiteTableMetadata(name string) (RuntimeSQLiteTableMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	schema, err := sqliteTableSchema(s.db, name)
	if err != nil {
		return RuntimeSQLiteTableMetadata{}, err
	}
	dbPath, err := sqliteDatabasePath(s.db)
	if err != nil {
		return RuntimeSQLiteTableMetadata{}, err
	}
	return RuntimeSQLiteTableMetadata{
		DB:           s.db,
		DatabasePath: dbPath,
		Table:        schema,
	}, nil
}

func sqliteTableExists(q sqliteQueryer, name string) (bool, error) {
	tableName, err := normalizeSQLiteIdentifier(name)
	if err != nil {
		return false, err
	}

	var count int
	if err := q.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, tableName).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func sqliteDatabasePath(q sqliteQueryer) (string, error) {
	rows, err := q.Query(`PRAGMA database_list`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			seq  int
			name string
			path string
		)
		if err := rows.Scan(&seq, &name, &path); err != nil {
			return "", err
		}
		if name == "main" {
			return path, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("sqlite main database path not found")
}

func sqliteTableSchema(q sqliteQueryer, name string) (SQLiteTableSchema, error) {
	tableName, err := normalizeSQLiteIdentifier(name)
	if err != nil {
		return SQLiteTableSchema{}, err
	}

	schema := SQLiteTableSchema{Name: tableName}

	var createSQL sql.NullString
	if err := q.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, tableName).Scan(&createSQL); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schema, nil
		}
		return SQLiteTableSchema{}, err
	}

	schema.Exists = true
	schema.CreateSQL = createSQL.String

	rows, err := q.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		return SQLiteTableSchema{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid int
			col SQLiteTableColumn
		)
		if err := rows.Scan(&cid, &col.Name, &col.Type, &col.NotNull, &col.DefaultValue, &col.PKOrdinal); err != nil {
			return SQLiteTableSchema{}, fmt.Errorf("scan %s schema: %w", tableName, err)
		}
		schema.Columns = append(schema.Columns, col)
	}
	if err := rows.Err(); err != nil {
		return SQLiteTableSchema{}, err
	}

	return schema, nil
}

func normalizeSQLiteIdentifier(name string) (string, error) {
	tableName := strings.TrimSpace(name)
	if tableName == "" {
		return "", fmt.Errorf("sqlite identifier is required")
	}
	for _, r := range tableName {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid sqlite identifier %q", tableName)
	}
	return tableName, nil
}

// --- Entry CRUD ---

// Register inserts or updates a single repository.
func (s *Store) Register(path, name, desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO brownfield_entries (type, key, name, desc, path) VALUES ('repo', ?, ?, ?, ?)
		ON CONFLICT(type, key) DO UPDATE SET
			name = COALESCE(NULLIF(excluded.name, ''), brownfield_entries.name),
			desc = COALESCE(NULLIF(excluded.desc, ''), brownfield_entries.desc)
	`, path, name, desc, path)
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
		INSERT INTO brownfield_entries (type, key, name, path) VALUES ('repo', ?, ?, ?)
		ON CONFLICT(type, key) DO UPDATE SET
			name = COALESCE(NULLIF(excluded.name, ''), brownfield_entries.name)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	var errs []string
	for _, r := range repos {
		if _, err := stmt.Exec(r.Path, r.Name, r.Path); err != nil {
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

// SyncMCPEntries synchronizes MCP entries using diff semantics:
// existing entries are preserved (rowid stable), new entries are inserted,
// and entries absent from the scan are deleted.
func (s *Store) SyncMCPEntries(servers []MCPServer) (int, error) {
	registeredAt := mcpSnapshotTimestamp()
	normalized := normalizeVisibleMCPServersForSnapshot(servers)

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Get existing MCP keys
	existingKeys := make(map[string]bool)
	rows, err := tx.Query(`SELECT key FROM brownfield_entries WHERE type = 'mcp'`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return 0, err
		}
		existingKeys[key] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Build scan key set from normalized servers (names already trimmed by normalizeVisibleMCPServersForSnapshot)
	scanServers := make(map[string]MCPServer, len(normalized))
	for _, server := range normalized {
		scanServers[server.Name] = server
	}

	// Delete stale entries (in DB but not in scan)
	for key := range existingKeys {
		if _, exists := scanServers[key]; !exists {
			if _, err := tx.Exec(`DELETE FROM brownfield_entries WHERE type = 'mcp' AND key = ?`, key); err != nil {
				return 0, fmt.Errorf("delete stale mcp %s: %w", key, err)
			}
		}
	}

	// Insert new entries in sorted order for deterministic rowid assignment
	sortedNames := make([]string, 0, len(scanServers))
	for name := range scanServers {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	for _, name := range sortedNames {
		server := scanServers[name]
		desc := normalizeMCPDescription(name, server.Desc)
		pathVal := normalizeOptionalPath(approvedSnapshotPath(server))
		if existingKeys[name] {
			// Update desc/path for existing entries (preserves rowid and is_default)
			if _, err := tx.Exec(`
				UPDATE brownfield_entries SET desc = ?, path = ?, registered_at = ?
				WHERE type = 'mcp' AND key = ?
			`, desc, pathVal, registeredAt, name); err != nil {
				return 0, fmt.Errorf("update mcp %s: %w", name, err)
			}
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO brownfield_entries (type, key, name, desc, path, is_default, registered_at)
			VALUES ('mcp', ?, ?, ?, ?, 0, ?)
		`, name, name, desc, pathVal, registeredAt); err != nil {
			return 0, fmt.Errorf("insert mcp %s: %w", name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(scanServers), nil
}

// ListEntries returns all entries (repo + mcp) with unified numbering.
func (s *Store) ListEntries(offset, limit int, defaultOnly bool) ([]Entry, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listEntries(offset, limit, defaultOnly, "")
}

func (s *Store) listEntries(offset, limit int, defaultOnly bool, typeFilter string) ([]Entry, int, error) {
	var whereParts []string
	var args []any
	if defaultOnly {
		whereParts = append(whereParts, "is_default = 1")
	}
	if typeFilter != "" {
		whereParts = append(whereParts, "type = ?")
		args = append(args, typeFilter)
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = "WHERE " + strings.Join(whereParts, " AND ")
	}

	var total int
	if err := s.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM brownfield_entries %s", whereClause), args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	effectiveLimit := limit
	if effectiveLimit <= 0 {
		effectiveLimit = 10000
	}

	selectArgs := append(append([]any{}, args...), effectiveLimit, offset)
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT rowid, type, key, name, COALESCE(desc, ''), COALESCE(path, ''), is_default, registered_at
		FROM brownfield_entries %s
		ORDER BY rowid ASC
		LIMIT ? OFFSET ?
	`, whereClause), selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.RowID, &e.Type, &e.Key, &e.Name, &e.Desc, &e.Path, &e.IsDefault, &e.RegisteredAt); err != nil {
			return nil, 0, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// List returns repos with pagination. If defaultOnly is true, returns only defaults.
func (s *Store) List(offset, limit int, defaultOnly bool) ([]Repo, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, total, err := s.listEntries(offset, limit, defaultOnly, "repo")
	if err != nil {
		return nil, 0, err
	}
	repos := make([]Repo, len(entries))
	for i, e := range entries {
		repos[i] = Repo{
			RowID:        e.RowID,
			Path:         e.Key,
			Name:         e.Name,
			Desc:         e.Desc,
			IsDefault:    e.IsDefault,
			RegisteredAt: e.RegisteredAt,
		}
	}
	return repos, total, nil
}

// UpdateDefault toggles the is_default flag for a repo by path.
func (s *Store) UpdateDefault(path string, isDefault bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec("UPDATE brownfield_entries SET is_default = ? WHERE type = 'repo' AND key = ?", isDefault, path)
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
// Works across both repo and mcp entries in the unified table.
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
		if err := tx.QueryRow("SELECT 1 FROM brownfield_entries WHERE rowid = ?", id).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("rowid %d does not exist", id)
			}
			return fmt.Errorf("validate rowid %d: %w", id, err)
		}
	}

	if _, err := tx.Exec("UPDATE brownfield_entries SET is_default = 0"); err != nil {
		return err
	}
	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		args := make([]interface{}, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			args[i] = id
		}
		q := fmt.Sprintf("UPDATE brownfield_entries SET is_default = 1 WHERE rowid IN (%s)", strings.Join(placeholders, ","))
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
	res, err := s.db.Exec("UPDATE brownfield_entries SET desc = ? WHERE type = 'repo' AND key = ?", desc, path)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("repo not found: %s", path)
	}
	return nil
}

// DefaultRepos returns all repos with is_default=1.
func (s *Store) DefaultRepos() ([]Repo, error) {
	repos, _, err := s.List(0, 0, true)
	return repos, err
}

// DefaultEntries returns all entries (repo + mcp) with is_default=1.
func (s *Store) DefaultEntries() ([]Entry, error) {
	entries, _, err := s.ListEntries(0, 0, true)
	return entries, err
}

// CountMCPs returns the number of MCP entries currently stored.
func (s *Store) CountMCPs() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM brownfield_entries WHERE type = 'mcp'`).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// ListMCPs returns MCP entries ordered by name.
func (s *Store) ListMCPs() ([]MCPServerSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, _, err := s.listEntries(0, 0, false, "mcp")
	if err != nil {
		return nil, err
	}
	mcps := make([]MCPServerSnapshot, 0, len(entries))
	for _, e := range entries {
		m := MCPServerSnapshot{
			Name:         e.Name,
			Desc:         e.Desc,
			IsDefault:    e.IsDefault,
			RegisteredAt: e.RegisteredAt,
		}
		if e.Path != "" {
			path := e.Path
			m.Path = &path
		}
		mcps = append(mcps, m)
	}
	return mcps, nil
}

// --- MCP normalization helpers (used by SyncMCPEntries and tests) ---

// snapshotRowsForMCPServers applies scan-time normalization for MCP snapshots.
func snapshotRowsForMCPServers(servers []MCPServer, registeredAt string) []MCPServerSnapshot {
	normalized := normalizeVisibleMCPServersForSnapshot(servers)
	snapshots := make([]MCPServerSnapshot, 0, len(normalized))
	for _, server := range normalized {
		name := strings.TrimSpace(server.Name)
		snapshots = append(snapshots, MCPServerSnapshot{
			Name:         name,
			Path:         approvedSnapshotPath(server),
			Desc:         normalizeMCPDescription(name, server.Desc),
			IsDefault:    defaultForMCPServer(server),
			RegisteredAt: registeredAt,
		})
	}
	return snapshots
}

func snapshotNamesForMCPServers(servers []MCPServer) []string {
	normalized := normalizeVisibleMCPServersForSnapshot(servers)
	names := make([]string, 0, len(normalized))
	for _, server := range normalized {
		names = append(names, strings.TrimSpace(server.Name))
	}
	return names
}

func mcpSnapshotTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func defaultForMCPServer(MCPServer) bool {
	return false
}

func normalizeOptionalPath(path *string) any {
	if path == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*path)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func approvedSnapshotPath(server MCPServer) *string {
	path := strings.TrimSpace(server.Path)
	if path == "" {
		return nil
	}
	return &path
}

func normalizeMCPDescription(name, desc string) string {
	trimmedDesc := strings.TrimSpace(desc)
	if trimmedDesc != "" {
		return trimmedDesc
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return ""
	}
	return fmt.Sprintf(unresolvedMCPDescriptionFormat, trimmedName)
}

// OpenStoreAt opens an existing brownfield SQLite database at the given path
// for read-only queries.
func OpenStoreAt(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&mode=ro")
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
