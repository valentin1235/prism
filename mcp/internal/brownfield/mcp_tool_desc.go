package brownfield

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const mcpToolMetadataResolutionTimeout = 2 * time.Second

type mcpToolListingClient interface {
	Initialize(context.Context, mcp.InitializeRequest) (*mcp.InitializeResult, error)
	ListTools(context.Context, mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
	Close() error
}

var newMCPToolListingClient = func(command string, args ...string) (mcpToolListingClient, error) {
	return mcpclient.NewStdioMCPClient(command, nil, args...)
}

func hydrateMCPServerDescriptions(ctx context.Context, servers []MCPServer) []MCPServer {
	if len(servers) == 0 {
		return nil
	}

	hydrated := make([]MCPServer, len(servers))
	copy(hydrated, servers)

	for i := range hydrated {
		if strings.TrimSpace(hydrated[i].Desc) != "" {
			continue
		}
		desc, err := resolveMCPServerToolMetadataDescription(ctx, hydrated[i])
		if err != nil {
			continue
		}
		hydrated[i].Desc = desc
	}
	return hydrated
}

func resolveMCPServerToolMetadataDescription(ctx context.Context, server MCPServer) (string, error) {
	commandText := strings.TrimSpace(server.Command)
	if commandText == "" {
		return "", fmt.Errorf("mcp server %q has no launch command", strings.TrimSpace(server.Name))
	}

	resolveCtx, cancel := context.WithTimeout(ctx, mcpToolMetadataResolutionTimeout)
	defer cancel()

	client, err := newMCPToolListingClient("/bin/sh", "-lc", commandText)
	if err != nil {
		return "", err
	}
	defer client.Close()

	initializeRequest := mcp.InitializeRequest{}
	initializeRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initializeRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "prism-brownfield-scan",
		Version: "1.0.0",
	}
	if _, err := client.Initialize(resolveCtx, initializeRequest); err != nil {
		return "", err
	}

	toolResult, err := client.ListTools(resolveCtx, mcp.ListToolsRequest{})
	if err != nil {
		return "", err
	}
	return summarizeResolvedMCPTools(toolResult.Tools), nil
}

func summarizeResolvedMCPTools(tools []mcp.Tool) string {
	type toolSummary struct {
		name     string
		fragment string
	}

	summaries := make([]toolSummary, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		fragment := summarizeResolvedMCPTool(tool)
		if name == "" || fragment == "" {
			continue
		}
		summaries = append(summaries, toolSummary{name: name, fragment: fragment})
	}
	if len(summaries) == 0 {
		return ""
	}

	sort.Slice(summaries, func(i, j int) bool {
		leftName := strings.ToLower(summaries[i].name)
		rightName := strings.ToLower(summaries[j].name)
		if leftName != rightName {
			return leftName < rightName
		}
		return summaries[i].fragment < summaries[j].fragment
	})

	fragments := make([]string, 0, len(summaries))
	seen := make(map[string]struct{}, len(summaries))
	for _, summary := range summaries {
		key := strings.ToLower(summary.fragment)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		fragments = append(fragments, summary.fragment)
	}
	if len(fragments) == 0 {
		return ""
	}

	const maxFragments = 3
	if len(fragments) > maxFragments {
		return fmt.Sprintf("%s; +%d more tools.", strings.Join(fragments[:maxFragments], "; "), len(fragments)-maxFragments)
	}
	return strings.Join(fragments, "; ") + "."
}

func summarizeResolvedMCPTool(tool mcp.Tool) string {
	if desc := strings.TrimSpace(tool.Description); desc != "" {
		return strings.TrimRight(desc, ".!? ")
	}
	return strings.TrimSpace(tool.Name)
}
