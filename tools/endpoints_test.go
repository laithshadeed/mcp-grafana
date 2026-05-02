//go:build unit
// +build unit

package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpgrafana "github.com/grafana/mcp-grafana"
)

func TestListEndpointsToolWithInstances(t *testing.T) {
	orig := mcpgrafana.GlobalInstanceStore()
	defer mcpgrafana.InitInstanceStore(orig.InstanceMap())

	mcpgrafana.InitInstanceStore(map[string]mcpgrafana.InstanceConfig{
		"alpha": {URL: "https://alpha.example.com", ServiceAccountToken: "tok"},
		"beta":  {URL: "https://beta.example.com", ServiceAccountToken: "tok"},
	})

	result, err := listEndpoints(context.Background(), listEndpointsParams{})
	require.NoError(t, err)
	assert.Contains(t, result, "alpha")
	assert.Contains(t, result, "https://alpha.example.com")
	assert.Contains(t, result, "beta")
	assert.Contains(t, result, "https://beta.example.com")
}

func TestListEndpointsToolWithoutInstances(t *testing.T) {
	orig := mcpgrafana.GlobalInstanceStore()
	defer mcpgrafana.InitInstanceStore(orig.InstanceMap())

	// Reset to nil
	mcpgrafana.ResetInstanceStore()

	result, err := listEndpoints(context.Background(), listEndpointsParams{})
	require.NoError(t, err)
	assert.Contains(t, result, "No instances configured")
}
