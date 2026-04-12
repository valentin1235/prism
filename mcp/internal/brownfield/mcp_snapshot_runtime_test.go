package brownfield

import (
	"database/sql"
	"os"
	"strings"
	"testing"
)

func runtimeSQLiteTableMetadata(t *testing.T, s *Store, name string) RuntimeSQLiteTableMetadata {
	t.Helper()

	metadata, err := s.RuntimeSQLiteTableMetadata(name)
	if err != nil {
		t.Fatalf("RuntimeSQLiteTableMetadata(%q): %v", name, err)
	}
	return metadata
}

func runtimeSQLiteMCPSnapshotCheck(t *testing.T, s *Store) MCPSnapshotRuntimeSQLiteCheck {
	t.Helper()

	check, err := s.RuntimeSQLiteMCPSnapshotCheck()
	if err != nil {
		t.Fatalf("RuntimeSQLiteMCPSnapshotCheck(): %v", err)
	}
	return check
}

func runtimeSQLiteTableSchema(t *testing.T, s *Store, name string) SQLiteTableSchema {
	t.Helper()
	return runtimeSQLiteTableMetadata(t, s, name).Table
}

func assertRuntimeSQLiteTableExists(t *testing.T, s *Store, name string) {
	t.Helper()

	if !runtimeSQLiteTableSchema(t, s, name).Exists {
		t.Fatalf("expected runtime table %q to exist", name)
	}
}

func runtimeSQLiteSnapshotNames(t *testing.T, s *Store) []string {
	t.Helper()

	metadata := runtimeSQLiteTableMetadata(t, s, mcpSnapshotTableName)
	rows, err := metadata.DB.Query(`
		SELECT name
		FROM mcp_server_snapshot
		ORDER BY name ASC
	`)
	if err != nil {
		t.Fatalf("query runtime snapshot names: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan runtime snapshot name: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate runtime snapshot names: %v", err)
	}
	return names
}

func assertRuntimeSQLiteSnapshotNames(t *testing.T, s *Store, want []string) {
	t.Helper()

	got := runtimeSQLiteSnapshotNames(t, s)
	if len(got) != len(want) {
		t.Fatalf("runtime sqlite snapshot row count = %d, want %d (%v)", len(got), len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("runtime sqlite snapshot names = %v, want %v", got, want)
		}
	}

	var duplicateNames int
	metadata := runtimeSQLiteTableMetadata(t, s, mcpSnapshotTableName)
	if err := metadata.DB.QueryRow(`
		SELECT COUNT(*)
		FROM (
			SELECT name
			FROM mcp_server_snapshot
			GROUP BY name
			HAVING COUNT(*) > 1
		)
	`).Scan(&duplicateNames); err != nil {
		t.Fatalf("count duplicate runtime snapshot names: %v", err)
	}
	if duplicateNames != 0 {
		t.Fatalf("duplicate runtime snapshot names = %d, want 0", duplicateNames)
	}
}

func runtimeSQLiteSnapshotRowByName(t *testing.T, s *Store, name string) MCPServerSnapshot {
	t.Helper()

	metadata := runtimeSQLiteTableMetadata(t, s, mcpSnapshotTableName)
	var (
		row    MCPServerSnapshot
		dbPath sql.NullString
	)
	if err := metadata.DB.QueryRow(`
		SELECT name, path, desc, is_default, registered_at
		FROM mcp_server_snapshot
		WHERE name = ?
	`, name).Scan(&row.Name, &dbPath, &row.Desc, &row.IsDefault, &row.RegisteredAt); err != nil {
		if err == sql.ErrNoRows {
			t.Fatalf("runtime sqlite snapshot row %q not found", name)
		}
		t.Fatalf("query runtime snapshot row %q: %v", name, err)
	}
	if dbPath.Valid {
		p := dbPath.String
		row.Path = &p
	}
	return row
}

func assertRuntimeSQLiteSnapshotDefaultsAllFalse(t *testing.T, s *Store) {
	t.Helper()

	metadata := runtimeSQLiteTableMetadata(t, s, mcpSnapshotTableName)
	rows, err := metadata.DB.Query(`
		SELECT name, is_default
		FROM mcp_server_snapshot
		ORDER BY name ASC
	`)
	if err != nil {
		t.Fatalf("query runtime snapshot defaults: %v", err)
	}
	defer rows.Close()

	var rowCount int
	for rows.Next() {
		var (
			name      string
			isDefault bool
		)
		if err := rows.Scan(&name, &isDefault); err != nil {
			t.Fatalf("scan runtime snapshot default row: %v", err)
		}
		rowCount++
		if isDefault {
			t.Fatalf("runtime sqlite snapshot row %q has is_default=true, want false when no MCP default source is configured", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate runtime snapshot defaults: %v", err)
	}
}

func assertRuntimeSQLiteSnapshotSharedRegisteredAt(t *testing.T, s *Store, wantCount int) string {
	t.Helper()

	metadata := runtimeSQLiteTableMetadata(t, s, mcpSnapshotTableName)
	rows, err := metadata.DB.Query(`
		SELECT name, registered_at
		FROM mcp_server_snapshot
		ORDER BY name ASC
	`)
	if err != nil {
		t.Fatalf("query runtime snapshot registered_at rows: %v", err)
	}
	defer rows.Close()

	var (
		rowCount           int
		sharedRegisteredAt string
	)
	for rows.Next() {
		var (
			name         string
			registeredAt string
		)
		if err := rows.Scan(&name, &registeredAt); err != nil {
			t.Fatalf("scan runtime snapshot registered_at row: %v", err)
		}
		rowCount++
		registeredAt = strings.TrimSpace(registeredAt)
		if registeredAt == "" {
			t.Fatalf("runtime snapshot row %q missing registered_at", name)
		}
		if sharedRegisteredAt == "" {
			sharedRegisteredAt = registeredAt
			continue
		}
		if registeredAt != sharedRegisteredAt {
			t.Fatalf("runtime snapshot row %q registered_at = %q, want shared scan timestamp %q", name, registeredAt, sharedRegisteredAt)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate runtime snapshot registered_at rows: %v", err)
	}
	if rowCount != wantCount {
		t.Fatalf("runtime snapshot registered_at row count = %d, want %d", rowCount, wantCount)
	}
	if sharedRegisteredAt == "" && wantCount > 0 {
		t.Fatal("expected non-empty shared runtime snapshot registered_at")
	}
	return sharedRegisteredAt
}

func assertRuntimeSQLiteMetadataUsesDatabasePath(t *testing.T, metadata RuntimeSQLiteTableMetadata, wantPath string) {
	t.Helper()

	gotPath := strings.TrimSpace(metadata.DatabasePath)
	if gotPath == "" {
		t.Fatal("expected runtime sqlite metadata to include a database path")
	}

	gotInfo, err := os.Stat(gotPath)
	if err != nil {
		t.Fatalf("stat runtime sqlite metadata path %q: %v", gotPath, err)
	}
	wantInfo, err := os.Stat(wantPath)
	if err != nil {
		t.Fatalf("stat expected runtime sqlite path %q: %v", wantPath, err)
	}
	if !os.SameFile(gotInfo, wantInfo) {
		t.Fatalf("runtime sqlite metadata path = %q, want same file as %q", gotPath, wantPath)
	}
}
