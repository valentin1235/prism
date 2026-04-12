package brownfield

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// MCPSnapshotRuntimeSQLiteCheck captures the authoritative runtime SQLite
// inspection result for the MCP snapshot table. Both summary-style schema
// verification and detailed evaluator assertions should consume this shared
// result so they cannot drift across different DB files or lookup paths.
type MCPSnapshotRuntimeSQLiteCheck struct {
	Metadata RuntimeSQLiteTableMetadata
	SchemaOK bool
	Shape    MCPSnapshotSchemaShapeCheck
}

// MCPSnapshotSchemaShapeCheck captures the schema verdict for the MCP snapshot
// table based on PRAGMA table_info introspection.
type MCPSnapshotSchemaShapeCheck struct {
	TableExists               bool
	ColumnCount               int
	NameColumnPrimaryKey      bool
	NameColumnNotNull         bool
	PathColumnPresent         bool
	PathColumnNullable        bool
	DescColumnNotNull         bool
	IsDefaultColumnNotNull    bool
	RegisteredAtColumnNotNull bool
	MatchesExpectedSchema     bool
}

const unresolvedMCPDescriptionFormat = "MCP server %s (tool metadata unavailable at scan time)"

const (
	mcpSnapshotTableName       = "mcp_server_snapshot"
	mcpSnapshotIndexName       = "ix_mcp_server_snapshot_is_default"
	legacyMCPSnapshotTableName = "brownfield_mcps"
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
	repoSchema := `
CREATE TABLE IF NOT EXISTS brownfield_repos (
    path TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    desc TEXT,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    registered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS ix_brownfield_repos_is_default ON brownfield_repos (is_default);
`
	if _, err := s.db.Exec(repoSchema); err != nil {
		return err
	}
	return s.ensureMCPTableSchema()
}

// EnsureMCPSnapshotTableSchema reruns the MCP snapshot migration against the
// live runtime SQLite handle. Scan handlers call this immediately before MCP
// snapshot writes so older runtime DB files are upgraded in place.
func (s *Store) EnsureMCPSnapshotTableSchema() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureMCPTableSchema()
}

func (s *Store) ensureMCPTableSchema() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ok, err := mcpSnapshotTableMatches(tx)
	if err != nil {
		return err
	}
	if ok {
		// Schema already matches — no migration needed. Rollback the read-only tx.
		return tx.Rollback()
	}

	if _, err := tx.Exec(fmt.Sprintf(`
DROP TABLE IF EXISTS %s;
DROP TABLE IF EXISTS %s;
CREATE TABLE %s (
    name TEXT NOT NULL PRIMARY KEY CHECK (length(trim(name)) > 0),
    path TEXT,
    desc TEXT NOT NULL CHECK (length(trim(desc)) > 0),
    is_default BOOLEAN NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    registered_at TIMESTAMP NOT NULL CHECK (length(trim(registered_at)) > 0)
);
CREATE INDEX IF NOT EXISTS %s ON %s (is_default);
`, mcpSnapshotTableName, legacyMCPSnapshotTableName, mcpSnapshotTableName, mcpSnapshotIndexName, mcpSnapshotTableName)); err != nil {
		return fmt.Errorf("ensure %s schema: %w", mcpSnapshotTableName, err)
	}

	return tx.Commit()
}

// VerifyMCPSnapshotTableSchema checks the live SQLite schema for the MCP
// snapshot table. Callers should use this for evaluator-facing schema checks so
// schema existence/results come from the same SQLite source of truth as the
// migration logic.
func (s *Store) VerifyMCPSnapshotTableSchema() error {
	check, err := s.RuntimeSQLiteMCPSnapshotCheck()
	if err != nil {
		return err
	}
	if !check.SchemaOK {
		return fmt.Errorf("%s schema does not match expected SQLite snapshot schema", mcpSnapshotTableName)
	}
	return nil
}

// RuntimeSQLiteMCPSnapshotCheck returns the runtime SQLite metadata plus the
// canonical schema verdict for the MCP snapshot table from the same migrated
// database instance used by scans.
func (s *Store) RuntimeSQLiteMCPSnapshotCheck() (MCPSnapshotRuntimeSQLiteCheck, error) {
	metadata, err := s.RuntimeSQLiteTableMetadata(mcpSnapshotTableName)
	if err != nil {
		return MCPSnapshotRuntimeSQLiteCheck{}, err
	}
	shape, err := inspectMCPSnapshotSchemaShape(metadata.Table)
	if err != nil {
		return MCPSnapshotRuntimeSQLiteCheck{}, err
	}
	return MCPSnapshotRuntimeSQLiteCheck{
		Metadata: metadata,
		SchemaOK: shape.MatchesExpectedSchema,
		Shape:    shape,
	}, nil
}

func mcpSnapshotTableMatches(tx *sql.Tx) (bool, error) {
	schema, err := sqliteTableSchema(tx, mcpSnapshotTableName)
	if err != nil {
		return false, err
	}
	return mcpSnapshotTableMatchesSchema(schema)
}

func mcpSnapshotTableMatchesSchema(schema SQLiteTableSchema) (bool, error) {
	shape, err := inspectMCPSnapshotSchemaShape(schema)
	if err != nil {
		return false, err
	}
	return shape.MatchesExpectedSchema, nil
}

func inspectMCPSnapshotSchemaShape(schema SQLiteTableSchema) (MCPSnapshotSchemaShapeCheck, error) {
	shape := MCPSnapshotSchemaShapeCheck{
		TableExists: schema.Exists,
		ColumnCount: len(schema.Columns),
	}
	if !schema.Exists {
		return shape, nil
	}

	type expectedCol struct {
		notNull   int
		pkOrdinal int
	}
	expected := map[string]expectedCol{
		"name":          {notNull: 1, pkOrdinal: 1},
		"path":          {notNull: 0, pkOrdinal: 0},
		"desc":          {notNull: 1, pkOrdinal: 0},
		"is_default":    {notNull: 1, pkOrdinal: 0},
		"registered_at": {notNull: 1, pkOrdinal: 0},
	}

	cols := make(map[string]SQLiteTableColumn, len(schema.Columns))
	for _, col := range schema.Columns {
		cols[col.Name] = col
	}

	if col, ok := cols["name"]; ok {
		shape.NameColumnNotNull = col.NotNull == expected["name"].notNull
		shape.NameColumnPrimaryKey = col.PKOrdinal == expected["name"].pkOrdinal
	}
	if col, ok := cols["path"]; ok {
		shape.PathColumnPresent = true
		shape.PathColumnNullable = col.NotNull == expected["path"].notNull
	}
	if col, ok := cols["desc"]; ok {
		shape.DescColumnNotNull = col.NotNull == expected["desc"].notNull
	}
	if col, ok := cols["is_default"]; ok {
		shape.IsDefaultColumnNotNull = col.NotNull == expected["is_default"].notNull
	}
	if col, ok := cols["registered_at"]; ok {
		shape.RegisteredAtColumnNotNull = col.NotNull == expected["registered_at"].notNull
	}

	shape.MatchesExpectedSchema =
		shape.ColumnCount == len(expected) &&
			shape.NameColumnPrimaryKey &&
			shape.NameColumnNotNull &&
			shape.PathColumnPresent &&
			shape.PathColumnNullable &&
			shape.DescColumnNotNull &&
			shape.IsDefaultColumnNotNull &&
			shape.RegisteredAtColumnNotNull

	return shape, nil
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
// Store connection. Evaluator-facing checks should use this helper so they
// inspect the same runtime SQLite source as the implementation.
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

	// Partial success by design: UPSERT means most failures are unusual (I/O, corruption).
	// Individual row errors are collected but don't abort the batch.
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

// ReplaceMCPsSnapshot replaces the MCP snapshot with the provided rows.
// Each call is authoritative for that scan: rows from prior scans are deleted
// before the current normalized snapshot is inserted, so servers no longer
// visible in `/mcp` do not survive a rescan.
func (s *Store) ReplaceMCPsSnapshot(servers []MCPServer) (int, error) {
	registeredAt := mcpSnapshotTimestamp()
	snapshots := snapshotRowsForMCPServers(servers, registeredAt)
	if err := s.ReplaceMCPs(snapshots); err != nil {
		return 0, err
	}
	return len(snapshots), nil
}

// snapshotRowsForMCPServers applies scan-time normalization for MCP snapshots.
// The inclusion basis is `/mcp` visibility only: every visible server with a
// non-empty trimmed name produces a snapshot row even if tool metadata
// resolution failed and Desc is blank. If multiple visible entries share the
// same trimmed server name, the survivor is selected with
// mcpSnapshotNameCollisionPolicy.description before persistence.
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
	// MCP scan snapshots currently have no separate default source of truth.
	// Recompute defaults on each scan as false until one exists.
	return false
}

// CountMCPs returns the number of MCP snapshot rows currently stored.
func (s *Store) CountMCPs() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var total int
	if err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, mcpSnapshotTableName)).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// ReplaceMCPs replaces the MCP snapshot table contents with the provided rows.
// The transaction always deletes the prior snapshot first, then inserts the
// current scan rows only. Empty or whitespace-only paths are treated as unknown
// and stored as NULL.
func (s *Store) ReplaceMCPs(mcps []MCPServerSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	type validatedSnapshotRow struct {
		name         string
		path         any
		desc         string
		isDefault    bool
		registeredAt string
	}

	rows := make([]validatedSnapshotRow, 0, len(mcps))
	for _, m := range mcps {
		name, desc, registeredAt, err := validateMCPServerSnapshotRow(m)
		if err != nil {
			return err
		}
		rows = append(rows, validatedSnapshotRow{
			name:         name,
			path:         normalizeOptionalPath(m.Path),
			desc:         desc,
			isDefault:    m.IsDefault,
			registeredAt: registeredAt,
		})
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM %s`, mcpSnapshotTableName)); err != nil {
		return err
	}

	stmt, err := tx.Prepare(fmt.Sprintf(`
		INSERT INTO %s (name, path, desc, is_default, registered_at)
		VALUES (?, ?, ?, ?, ?)
	`, mcpSnapshotTableName))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, row := range rows {
		if _, err := stmt.Exec(row.name, row.path, row.desc, row.isDefault, row.registeredAt); err != nil {
			return fmt.Errorf("register mcp %s: %w", row.name, err)
		}
	}

	return tx.Commit()
}

func validateMCPServerSnapshotRow(row MCPServerSnapshot) (string, string, string, error) {
	name := strings.TrimSpace(row.Name)
	if name == "" {
		return "", "", "", fmt.Errorf("mcp name is required")
	}

	desc := strings.TrimSpace(row.Desc)
	if desc == "" {
		return "", "", "", fmt.Errorf("mcp desc is required for %s", name)
	}

	registeredAt := strings.TrimSpace(row.RegisteredAt)
	if registeredAt == "" {
		return "", "", "", fmt.Errorf("mcp registered_at is required for %s", name)
	}

	return name, desc, registeredAt, nil
}

// ListMCPs returns the latest MCP scan snapshot ordered by name.
func (s *Store) ListMCPs() ([]MCPServerSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT name, path, desc, is_default, registered_at
		FROM mcp_server_snapshot
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mcps []MCPServerSnapshot
	for rows.Next() {
		var (
			m      MCPServerSnapshot
			dbPath sql.NullString
		)
		if err := rows.Scan(&m.Name, &dbPath, &m.Desc, &m.IsDefault, &m.RegisteredAt); err != nil {
			return nil, fmt.Errorf("scan mcp row: %w", err)
		}
		if dbPath.Valid {
			path := dbPath.String
			m.Path = &path
		}
		mcps = append(mcps, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mcps, nil
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

// approvedSnapshotPath persists only ontology-approved MCP path metadata.
// Today the only approved source is MCPServer.Path as populated from explicit
// runtime metadata (for example an absolute transport command path). When that
// metadata is missing or unresolved, the snapshot stores SQL NULL.
func approvedSnapshotPath(server MCPServer) *string {
	path := strings.TrimSpace(server.Path)
	if path == "" {
		return nil
	}
	return &path
}

// normalizeMCPDescription applies the brownfield MCP description ontology:
// use resolved tool metadata verbatim after trimming, otherwise emit the exact
// deterministic unresolved fallback format expected by external evaluators.
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
		ORDER BY rowid ASC
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

// DefaultRepos returns all repos with is_default=1.
// Returns an empty slice (not error) if no defaults are set.
func (s *Store) DefaultRepos() ([]Repo, error) {
	repos, _, err := s.List(0, 0, true)
	return repos, err
}

// OpenStoreAt opens an existing brownfield SQLite database at the given path
// for read-only queries. Unlike NewStore, it skips DDL initialization and
// opens in read-only mode. Callers must verify the file exists before calling.
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
