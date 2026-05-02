package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/server"

	mcpgrafana "github.com/grafana/mcp-grafana"
)

type listEndpointsParams struct{}

func listEndpoints(_ context.Context, _ listEndpointsParams) (string, error) {
	store := mcpgrafana.GlobalInstanceStore()
	if store == nil || !store.HasInstances() {
		return "No instances configured. Set GRAFANA_INSTANCES_FILE to enable multi-instance mode.", nil
	}
	instances := store.ListInstances()
	result := ""
	for _, inst := range instances {
		result += inst.Name + " → " + inst.URL + "\n"
	}
	return result, nil
}

// AddEndpointsTools registers the list_endpoints tool.
func AddEndpointsTools(s *server.MCPServer) {
	tool := mcpgrafana.MustTool(
		"list_endpoints",
		"List all available Grafana instances that can be used as the 'endpoint' parameter in other tools.",
		listEndpoints,
	)
	tool.Register(s)
}
