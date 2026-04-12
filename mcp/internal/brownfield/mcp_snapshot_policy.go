package brownfield

import (
	"sort"
	"strings"
)

type mcpSnapshotNameCollisionPolicySpec struct {
	id          string
	description string
	choose      func(current, candidate MCPServer) MCPServer
}

type mcpSnapshotOntologySpec struct {
	nameCollisionPolicyID          string
	nameCollisionPolicyDescription string
}

// mcpSnapshotOntology is the scan ontology for authoritative MCP snapshot
// persistence. The documented name collision policy must stay identical to the
// runtime survivor rule used during normalization.
var mcpSnapshotOntology = mcpSnapshotOntologySpec{
	nameCollisionPolicyID:          "prefer_approved_path_then_resolved_description_then_lexicographically_smallest_normalized_snapshot_fingerprint",
	nameCollisionPolicyDescription: "prefer approved path, then resolved description, then lexicographically smallest normalized snapshot fingerprint",
}

// mcpSnapshotNameCollisionPolicy is the single documented source of truth for
// duplicate visible `/mcp` server names. Snapshot normalization, persistence,
// and tests must route survivor selection through this policy instead of
// re-encoding the rule elsewhere.
var mcpSnapshotNameCollisionPolicy = mcpSnapshotNameCollisionPolicySpec{
	id:          mcpSnapshotOntology.nameCollisionPolicyID,
	description: mcpSnapshotOntology.nameCollisionPolicyDescription,
	choose: func(current, candidate MCPServer) MCPServer {
		if compareMCPServerSnapshotCandidates(candidate, current) < 0 {
			return candidate
		}
		return current
	},
}

// normalizeVisibleMCPServersForSnapshot applies the scan contract before any
// SQLite writes occur. Snapshot membership is driven only by `/mcp` visibility:
// explicitly hidden entries are excluded, entries with blank names are ignored,
// and duplicate trimmed names are resolved with
// mcpSnapshotNameCollisionPolicy.description.
func normalizeVisibleMCPServersForSnapshot(servers []MCPServer) []MCPServer {
	byName := make(map[string]MCPServer, len(servers))
	for _, server := range servers {
		if !mcpServerVisibleAtScan(server) {
			continue
		}
		name := strings.TrimSpace(server.Name)
		if name == "" {
			continue
		}
		server.Name = name
		if existing, ok := byName[name]; ok {
			byName[name] = mcpSnapshotNameCollisionPolicy.choose(existing, server)
			continue
		}
		byName[name] = server
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	normalized := make([]MCPServer, 0, len(names))
	for _, name := range names {
		normalized = append(normalized, byName[name])
	}
	return normalized
}

// mcpServerVisibleAtScan determines snapshot inclusion. When VisibilityOK is
// false (visibility could not be determined, e.g. zero-value struct), the
// server is included by default so discovery sources that don't set
// VisibilityOK don't silently exclude servers.
func mcpServerVisibleAtScan(server MCPServer) bool {
	if !server.VisibilityOK {
		return true
	}
	return server.Visible
}

func compareMCPServerSnapshotCandidates(left, right MCPServer) int {
	leftPath, leftHasPath := approvedSnapshotPathValue(left)
	rightPath, rightHasPath := approvedSnapshotPathValue(right)
	if leftHasPath != rightHasPath {
		if leftHasPath {
			return -1
		}
		return 1
	}

	leftHasResolvedDesc := strings.TrimSpace(left.Desc) != ""
	rightHasResolvedDesc := strings.TrimSpace(right.Desc) != ""
	if leftHasResolvedDesc != rightHasResolvedDesc {
		if leftHasResolvedDesc {
			return -1
		}
		return 1
	}

	if cmp := strings.Compare(leftPath, rightPath); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(
		normalizeMCPDescription(strings.TrimSpace(left.Name), left.Desc),
		normalizeMCPDescription(strings.TrimSpace(right.Name), right.Desc),
	); cmp != 0 {
		return cmp
	}
	return strings.Compare(strings.TrimSpace(left.Desc), strings.TrimSpace(right.Desc))
}

func approvedSnapshotPathValue(server MCPServer) (string, bool) {
	path := approvedSnapshotPath(server)
	if path == nil {
		return "", false
	}
	return *path, true
}
