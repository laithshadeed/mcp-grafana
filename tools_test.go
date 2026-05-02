//go:build unit
// +build unit

package mcpgrafana

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testToolParams struct {
	Name     string `json:"name" jsonschema:"required,description=The name parameter"`
	Value    int    `json:"value" jsonschema:"required,description=The value parameter"`
	Optional bool   `json:"optional,omitempty" jsonschema:"description=An optional parameter"`
}

func testToolHandler(ctx context.Context, params testToolParams) (*mcp.CallToolResult, error) {
	if params.Name == "error" {
		return nil, errors.New("test error")
	}
	return mcp.NewToolResultText(params.Name + ": " + string(rune(params.Value))), nil
}

type emptyToolParams struct{}

func emptyToolHandler(ctx context.Context, params emptyToolParams) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("empty"), nil
}

// New handlers for different return types
func stringToolHandler(ctx context.Context, params testToolParams) (string, error) {
	if params.Name == "error" {
		return "", errors.New("test error")
	}
	if params.Name == "empty" {
		return "", nil
	}
	return params.Name + ": " + string(rune(params.Value)), nil
}

func stringPtrToolHandler(ctx context.Context, params testToolParams) (*string, error) {
	if params.Name == "error" {
		return nil, errors.New("test error")
	}
	if params.Name == "nil" {
		return nil, nil
	}
	if params.Name == "empty" {
		empty := ""
		return &empty, nil
	}
	result := params.Name + ": " + string(rune(params.Value))
	return &result, nil
}

type TestResult struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func structToolHandler(ctx context.Context, params testToolParams) (TestResult, error) {
	if params.Name == "error" {
		return TestResult{}, errors.New("test error")
	}
	return TestResult{
		Name:  params.Name,
		Value: params.Value,
	}, nil
}

func structPtrToolHandler(ctx context.Context, params testToolParams) (*TestResult, error) {
	if params.Name == "error" {
		return nil, errors.New("test error")
	}
	if params.Name == "nil" {
		return nil, nil
	}
	return &TestResult{
		Name:  params.Name,
		Value: params.Value,
	}, nil
}

func sliceToolHandler(ctx context.Context, params testToolParams) ([]TestResult, error) {
	if params.Name == "error" {
		return nil, errors.New("test error")
	}
	if params.Name == "nil" {
		return nil, nil
	}
	if params.Name == "empty" {
		return []TestResult{}, nil
	}
	return []TestResult{{Name: params.Name, Value: params.Value}}, nil
}

func TestConvertTool(t *testing.T) {
	t.Run("valid handler conversion", func(t *testing.T) {
		tool, handler, err := ConvertTool("test_tool", "A test tool", testToolHandler)

		require.NoError(t, err)
		require.NotNil(t, tool)
		require.NotNil(t, handler)

		// Check tool properties
		assert.Equal(t, "test_tool", tool.Name)
		assert.Equal(t, "A test tool", tool.Description)

		// Check schema properties by marshaling the tool
		toolJSON, err := json.Marshal(tool)
		require.NoError(t, err)

		var toolData map[string]any
		err = json.Unmarshal(toolJSON, &toolData)
		require.NoError(t, err)

		inputSchema, ok := toolData["inputSchema"].(map[string]any)
		require.True(t, ok, "inputSchema should be a map")
		assert.Equal(t, "object", inputSchema["type"])

		properties, ok := inputSchema["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map")
		assert.Contains(t, properties, "name")
		assert.Contains(t, properties, "value")
		assert.Contains(t, properties, "optional")

		// Test handler execution
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "test_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test: A", resultString.Text)

		// Test error handling
		errorRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "test_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 66,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("empty handler params", func(t *testing.T) {
		tool, handler, err := ConvertTool("empty", "description", emptyToolHandler)

		require.NoError(t, err)
		require.NotNil(t, tool)
		require.NotNil(t, handler)

		// Check tool properties
		assert.Equal(t, "empty", tool.Name)
		assert.Equal(t, "description", tool.Description)

		// Check schema properties by marshaling the tool
		toolJSON, err := json.Marshal(tool)
		require.NoError(t, err)

		var toolData map[string]any
		err = json.Unmarshal(toolJSON, &toolData)
		require.NoError(t, err)

		inputSchema, ok := toolData["inputSchema"].(map[string]any)
		require.True(t, ok, "inputSchema should be a map")
		assert.Equal(t, "object", inputSchema["type"])

		properties, ok := inputSchema["properties"].(map[string]any)
		require.True(t, ok, "properties should be a map")
		assert.Len(t, properties, 0)

		// Test handler execution
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "empty",
			},
		}
		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "empty", resultString.Text)
	})

	t.Run("string return type", func(t *testing.T) {
		_, handler, err := ConvertTool("string_tool", "A string tool", stringToolHandler)
		require.NoError(t, err)

		// Test normal string return
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "string_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test: A", resultString.Text)

		// Test empty string return
		emptyRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "string_tool",
				Arguments: map[string]any{
					"name":  "empty",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, emptyRequest)
		require.NoError(t, err)
		require.NotNil(t, result, "empty string should return non-nil result to prevent mcp-go crash")
		require.Len(t, result.Content, 1)
		emptyText, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "", emptyText.Text)

		// Test error return
		errorRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "string_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("string pointer return type", func(t *testing.T) {
		_, handler, err := ConvertTool("string_ptr_tool", "A string pointer tool", stringPtrToolHandler)
		require.NoError(t, err)

		// Test normal string pointer return
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "string_ptr_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test: A", resultString.Text)

		// Test nil string pointer return
		nilRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "string_ptr_tool",
				Arguments: map[string]any{
					"name":  "nil",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, nilRequest)
		require.NoError(t, err)
		require.NotNil(t, result, "nil pointer should return a non-nil result to prevent mcp-go crash")
		require.Len(t, result.Content, 1)
		nullText, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "null", nullText.Text)

		// Test empty string pointer return
		emptyRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "string_ptr_tool",
				Arguments: map[string]any{
					"name":  "empty",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, emptyRequest)
		require.NoError(t, err)
		require.NotNil(t, result, "empty *string should return non-nil result to prevent mcp-go crash")
		require.Len(t, result.Content, 1)
		emptyText, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "", emptyText.Text)

		// Test error return
		errorRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "string_ptr_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("struct return type", func(t *testing.T) {
		_, handler, err := ConvertTool("struct_tool", "A struct tool", structToolHandler)
		require.NoError(t, err)

		// Test normal struct return
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "struct_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, resultString.Text, `"name":"test"`)
		assert.Contains(t, resultString.Text, `"value":65`)

		// Test error return
		errorRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "struct_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("struct pointer return type", func(t *testing.T) {
		_, handler, err := ConvertTool("struct_ptr_tool", "A struct pointer tool", structPtrToolHandler)
		require.NoError(t, err)

		// Test normal struct pointer return
		ctx := context.Background()
		request := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "struct_ptr_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 65, // ASCII 'A'
				},
			},
		}

		result, err := handler(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, resultString.Text, `"name":"test"`)
		assert.Contains(t, resultString.Text, `"value":65`)

		// Test nil struct pointer return
		nilRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "struct_ptr_tool",
				Arguments: map[string]any{
					"name":  "nil",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, nilRequest)
		require.NoError(t, err)
		require.NotNil(t, result, "nil pointer should return a non-nil result to prevent mcp-go crash")
		require.Len(t, result.Content, 1)
		nullText, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "null", nullText.Text)

		// Test error return
		errorRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "struct_ptr_tool",
				Arguments: map[string]any{
					"name":  "error",
					"value": 65,
				},
			},
		}

		result, err = handler(ctx, errorRequest)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "test error", resultString.Text)
	})

	t.Run("slice return type - nil slice returns non-nil result", func(t *testing.T) {
		// This test verifies the fix for https://github.com/grafana/mcp-grafana/issues/660
		// where a nil slice return caused a nil pointer dereference in mcp-go.
		_, handler, err := ConvertTool("slice_tool", "A slice tool", sliceToolHandler)
		require.NoError(t, err)

		ctx := context.Background()

		// Test nil slice return - must NOT return nil result (causes mcp-go crash)
		nilRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "slice_tool",
				Arguments: map[string]any{
					"name":  "nil",
					"value": 1,
				},
			},
		}

		result, err := handler(ctx, nilRequest)
		require.NoError(t, err)
		require.NotNil(t, result, "nil slice must return non-nil result to prevent mcp-go nil pointer dereference")

		// Test empty slice return - should return valid JSON array
		emptyRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "slice_tool",
				Arguments: map[string]any{
					"name":  "empty",
					"value": 1,
				},
			},
		}

		result, err = handler(ctx, emptyRequest)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "[]", resultString.Text)

		// Test normal slice return
		normalRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "slice_tool",
				Arguments: map[string]any{
					"name":  "test",
					"value": 42,
				},
			},
		}

		result, err = handler(ctx, normalRequest)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)
		resultString, ok = result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, resultString.Text, `"name":"test"`)
		assert.Contains(t, resultString.Text, `"value":42`)
	})

	t.Run("invalid handler types", func(t *testing.T) {
		// Test wrong second argument type (not a struct)
		wrongSecondArgFunc := func(ctx context.Context, s string) (*mcp.CallToolResult, error) {
			return nil, nil
		}
		_, _, err := ConvertTool("invalid", "description", wrongSecondArgFunc)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "second argument must be a struct")
	})

	t.Run("handler execution with invalid arguments", func(t *testing.T) {
		_, handler, err := ConvertTool("test_tool", "A test tool", testToolHandler)
		require.NoError(t, err)

		// Test with invalid JSON
		invalidRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"name": make(chan int), // Channels can't be marshaled to JSON
				},
			},
		}

		_, err = handler(context.Background(), invalidRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "marshal args")

		// Test with type mismatch
		mismatchRequest := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Arguments: map[string]any{
					"name":  123, // Should be a string
					"value": "not an int",
				},
			},
		}

		_, err = handler(context.Background(), mismatchRequest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal args")
	})
}

func TestCreateJSONSchemaFromHandler(t *testing.T) {
	schema := createJSONSchemaFromHandler(testToolHandler)

	assert.Equal(t, "object", schema.Type)
	assert.Len(t, schema.Required, 2) // name and value are required, optional is not

	// Check properties
	nameProperty, ok := schema.Properties.Get("name")
	assert.True(t, ok)
	assert.Equal(t, "string", nameProperty.Type)
	assert.Equal(t, "The name parameter", nameProperty.Description)

	valueProperty, ok := schema.Properties.Get("value")
	assert.True(t, ok)
	assert.Equal(t, "integer", valueProperty.Type)
	assert.Equal(t, "The value parameter", valueProperty.Description)

	optionalProperty, ok := schema.Properties.Get("optional")
	assert.True(t, ok)
	assert.Equal(t, "boolean", optionalProperty.Type)
	assert.Equal(t, "An optional parameter", optionalProperty.Description)
}

func TestEmptyStructJSONSchema(t *testing.T) {
	// Test that empty structs generate correct JSON schema with empty properties object
	tool, _, err := ConvertTool("empty_tool", "An empty tool", emptyToolHandler)
	require.NoError(t, err)

	// Marshal the entire Tool to JSON
	jsonBytes, err := json.Marshal(tool)
	require.NoError(t, err)
	t.Logf("Marshaled Tool JSON: %s", string(jsonBytes))

	// Unmarshal to verify structure
	var unmarshaled map[string]any
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	// Verify that inputSchema exists
	inputSchema, exists := unmarshaled["inputSchema"]
	assert.True(t, exists, "inputSchema field should exist in tool JSON")
	assert.NotNil(t, inputSchema, "inputSchema should not be nil")

	// Verify inputSchema structure
	inputSchemaMap, ok := inputSchema.(map[string]any)
	assert.True(t, ok, "inputSchema should be a map")

	// Verify type is object
	assert.Equal(t, "object", inputSchemaMap["type"], "inputSchema type should be object")

	// Verify that properties key exists and is an empty object
	properties, exists := inputSchemaMap["properties"]
	assert.True(t, exists, "properties field should exist in inputSchema")
	assert.NotNil(t, properties, "properties should not be nil")

	propertiesMap, ok := properties.(map[string]any)
	assert.True(t, ok, "properties should be a map")
	assert.Len(t, propertiesMap, 0, "properties should be an empty map")
}

func TestValidateNoBooleanSchemas(t *testing.T) {
	t.Run("rejects bare true in properties", func(t *testing.T) {
		input := `{"type":"object","properties":{"model":true,"name":{"type":"string"}}}`
		err := validateNoBooleanSchemas("test_tool", []byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bare boolean schema")
		assert.Contains(t, err.Error(), "test_tool")
		assert.Contains(t, err.Error(), "properties.model")
	})

	t.Run("rejects bare false in properties", func(t *testing.T) {
		input := `{"type":"object","properties":{"blocked":false}}`
		err := validateNoBooleanSchemas("test_tool", []byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bare boolean schema")
	})

	t.Run("rejects bare true in additionalProperties", func(t *testing.T) {
		input := `{"type":"object","properties":{},"additionalProperties":true}`
		err := validateNoBooleanSchemas("test_tool", []byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "additionalProperties")
	})

	t.Run("rejects bare true in items", func(t *testing.T) {
		input := `{"type":"array","items":true}`
		err := validateNoBooleanSchemas("test_tool", []byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "items")
	})

	t.Run("rejects nested bare booleans", func(t *testing.T) {
		// Simulates the alert rule schema: properties -> data -> items -> properties -> model: true
		input := `{
			"type": "object",
			"properties": {
				"data": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"model": true,
							"refId": {"type": "string"}
						}
					}
				}
			}
		}`
		err := validateNoBooleanSchemas("test_tool", []byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model")
	})

	t.Run("rejects bare booleans in allOf/anyOf/oneOf", func(t *testing.T) {
		input := `{"allOf":[true,{"type":"string"}]}`
		err := validateNoBooleanSchemas("test_tool", []byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allOf")
	})

	t.Run("accepts valid schemas without bare booleans", func(t *testing.T) {
		input := `{
			"type": "object",
			"properties": {
				"name": {"type": "string", "description": "A name"},
				"count": {"type": "integer"},
				"tags": {"type": "array", "items": {"type": "string"}}
			},
			"required": ["name"]
		}`
		err := validateNoBooleanSchemas("test_tool", []byte(input))
		assert.NoError(t, err)
	})

	t.Run("does not flag non-schema boolean values", func(t *testing.T) {
		// uniqueItems: true is a non-schema boolean — should be allowed
		input := `{"type":"array","items":{"type":"string"},"uniqueItems":true}`
		err := validateNoBooleanSchemas("test_tool", []byte(input))
		assert.NoError(t, err)
	})

	t.Run("error message points to Mapper as the fix", func(t *testing.T) {
		input := `{"type":"object","properties":{"data":true}}`
		err := validateNoBooleanSchemas("my_tool", []byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "my_tool")
		assert.Contains(t, err.Error(), "Mapper")
		assert.Contains(t, err.Error(), "interface{}")
	})
}

// interfaceFieldParams tests that tools with interface{} fields produce
// valid schemas (the Mapper converts them to empty object schemas).
type interfaceFieldParams struct {
	Name  string `json:"name" jsonschema:"required,description=A name"`
	Model any    `json:"model" jsonschema:"description=An arbitrary model"`
}

func interfaceFieldHandler(ctx context.Context, params interfaceFieldParams) (string, error) {
	return params.Name, nil
}

func TestConvertToolHandlesInterfaceFields(t *testing.T) {
	// The Mapper in the reflector should convert interface{}/any fields to
	// empty object schemas {}, so ConvertTool should succeed (not error).
	tool, _, err := ConvertTool("interface_tool", "Tool with interface field", interfaceFieldHandler)
	require.NoError(t, err)

	// Verify the schema contains an object for the model field, not bare true
	var schema map[string]any
	err = json.Unmarshal(tool.RawInputSchema, &schema)
	require.NoError(t, err)

	props := schema["properties"].(map[string]any)
	model := props["model"]
	modelObj, ok := model.(map[string]any)
	require.True(t, ok, "model property should be an object schema, got %T", model)
	t.Logf("model schema: %v", modelObj)
}

func TestConvertToolEndpointInjectionInSchema(t *testing.T) {
	orig := globalInstanceStore
	defer func() { globalInstanceStore = orig }()

	globalInstanceStore = &InstanceStore{
		instances: map[string]InstanceConfig{
			"cde":  {URL: "https://grafana.cde.example.com", ServiceAccountToken: "tok-cde"},
			"edge": {URL: "https://grafana.edge.example.com", ServiceAccountToken: "tok-edge"},
		},
		clients: make(map[string]*GrafanaClient),
	}

	tool, _, err := ConvertTool("test_tool", "A test tool", testToolHandler)
	require.NoError(t, err)

	var schema map[string]any
	err = json.Unmarshal(tool.RawInputSchema, &schema)
	require.NoError(t, err)

	props := schema["properties"].(map[string]any)
	endpoint, ok := props["endpoint"]
	require.True(t, ok, "endpoint property should be injected when InstanceStore is active")

	epMap := endpoint.(map[string]any)
	assert.Equal(t, "string", epMap["type"])
	enum, ok := epMap["enum"].([]any)
	require.True(t, ok, "endpoint should have enum")
	assert.Contains(t, enum, "cde")
	assert.Contains(t, enum, "edge")
}

func TestConvertToolNoEndpointWithoutInstanceStore(t *testing.T) {
	orig := globalInstanceStore
	defer func() { globalInstanceStore = orig }()

	globalInstanceStore = nil

	tool, _, err := ConvertTool("test_tool", "A test tool", testToolHandler)
	require.NoError(t, err)

	var schema map[string]any
	err = json.Unmarshal(tool.RawInputSchema, &schema)
	require.NoError(t, err)

	props := schema["properties"].(map[string]any)
	_, hasEndpoint := props["endpoint"]
	assert.False(t, hasEndpoint, "endpoint property should NOT be injected without InstanceStore")
}

func TestConvertToolEndpointResolution(t *testing.T) {
	orig := globalInstanceStore
	defer func() { globalInstanceStore = orig }()

	instances := map[string]InstanceConfig{
		"edge": {URL: "https://grafana.edge.example.com", ServiceAccountToken: "tok-edge"},
	}
	globalInstanceStore = &InstanceStore{
		instances: instances,
		clients:   make(map[string]*GrafanaClient),
	}

	// Create a tool that reads the config from context to verify endpoint resolution
	type verifyParams struct {
		Message string `json:"message" jsonschema:"required"`
	}
	verifyHandler := func(ctx context.Context, p verifyParams) (string, error) {
		cfg := GrafanaConfigFromContext(ctx)
		return "url=" + cfg.URL, nil
	}

	_, handler, err := ConvertTool("verify", "verify tool", verifyHandler)
	require.NoError(t, err)

	ctx := context.Background()
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "verify",
			Arguments: map[string]any{
				"endpoint": "edge",
				"message":  "hello",
			},
		},
	}

	result, err := handler(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	text := result.Content[0].(mcp.TextContent).Text
	assert.Contains(t, text, "url=https://grafana.edge.example.com")
}

func TestConvertToolEndpointResolutionUnknownEndpoint(t *testing.T) {
	orig := globalInstanceStore
	defer func() { globalInstanceStore = orig }()

	globalInstanceStore = &InstanceStore{
		instances: map[string]InstanceConfig{
			"cde": {URL: "https://grafana.cde.example.com", ServiceAccountToken: "tok-cde"},
		},
		clients: make(map[string]*GrafanaClient),
	}

	_, handler, err := ConvertTool("test_tool", "A test tool", testToolHandler)
	require.NoError(t, err)

	ctx := context.Background()
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "test_tool",
			Arguments: map[string]any{
				"endpoint": "nonexistent",
				"name":     "test",
				"value":    65,
			},
		},
	}

	_, err = handler(ctx, request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

// TestRegisterInjectsEndpointAfterStoreInit reproduces the package-init race
// that was hiding the endpoint param from every tool except list_endpoints.
//
// Real layout in the wild:
//
//	var ListDatasources = mcpgrafana.MustTool(...)   // ConvertTool runs at init
//	func main() {
//	    InitInstanceStore(...)                       // store populated AFTER init
//	    ...
//	    AddDatasourceTools(s)                        // calls ListDatasources.Register(s)
//	}
//
// At ConvertTool time the store is nil, so the schema is frozen without
// "endpoint". The fix injects at Register time, after the store is up.
func TestRegisterInjectsEndpointAfterStoreInit(t *testing.T) {
	orig := globalInstanceStore
	defer func() { globalInstanceStore = orig }()

	// Step 1: ConvertTool runs FIRST, with no store — mimics package init.
	globalInstanceStore = nil
	tool := MustTool("test_tool", "A test tool", testToolHandler)

	// Confirm the cached schema has no "endpoint" yet.
	var pre map[string]any
	require.NoError(t, json.Unmarshal(tool.Tool.RawInputSchema, &pre))
	preProps := pre["properties"].(map[string]any)
	_, hasEndpoint := preProps["endpoint"]
	require.False(t, hasEndpoint, "precondition: schema before store init should not have endpoint")

	// Step 2: Initialize the store, then Register — mimics main() after init.
	globalInstanceStore = &InstanceStore{
		instances: map[string]InstanceConfig{
			"cde":  {URL: "https://grafana.cde.example.com", ServiceAccountToken: "tok-cde"},
			"edge": {URL: "https://grafana.edge.example.com", ServiceAccountToken: "tok-edge"},
		},
		clients: make(map[string]*GrafanaClient),
	}

	srv := server.NewMCPServer("test", "0")
	tool.Register(srv)

	// Step 3: After Register, the schema served by tools/list must have endpoint.
	var post map[string]any
	require.NoError(t, json.Unmarshal(tool.Tool.RawInputSchema, &post))
	postProps := post["properties"].(map[string]any)
	endpoint, ok := postProps["endpoint"]
	require.True(t, ok, "Register must inject endpoint when GlobalInstanceStore is populated")

	epMap := endpoint.(map[string]any)
	assert.Equal(t, "string", epMap["type"])
	enum := epMap["enum"].([]any)
	assert.Contains(t, enum, "cde")
	assert.Contains(t, enum, "edge")

	// Pre-existing properties must be preserved.
	for _, want := range []string{"name", "value", "optional"} {
		_, kept := postProps[want]
		assert.True(t, kept, "Register should preserve original property %q", want)
	}
}

// TestRegisterIdempotentEndpointInject covers the case where ConvertTool
// already injected (because the store happened to be set at that time) — a
// second pass at Register must not corrupt the schema.
func TestRegisterIdempotentEndpointInject(t *testing.T) {
	orig := globalInstanceStore
	defer func() { globalInstanceStore = orig }()

	globalInstanceStore = &InstanceStore{
		instances: map[string]InstanceConfig{
			"cde": {URL: "https://grafana.cde.example.com", ServiceAccountToken: "tok-cde"},
		},
		clients: make(map[string]*GrafanaClient),
	}

	tool := MustTool("test_tool", "A test tool", testToolHandler)
	srv := server.NewMCPServer("test", "0")
	tool.Register(srv)

	var schema map[string]any
	require.NoError(t, json.Unmarshal(tool.Tool.RawInputSchema, &schema))
	props := schema["properties"].(map[string]any)
	endpoint, ok := props["endpoint"]
	require.True(t, ok)
	epMap := endpoint.(map[string]any)
	enum := epMap["enum"].([]any)
	assert.Equal(t, []any{"cde"}, enum, "enum should reflect the store at Register time, not be doubled or stale")
}

// TestRegisterDoesNothingWithoutStore confirms the no-multi-instance path
// still produces a schema without endpoint (no spurious enum on plain installs).
func TestRegisterDoesNothingWithoutStore(t *testing.T) {
	orig := globalInstanceStore
	defer func() { globalInstanceStore = orig }()

	globalInstanceStore = nil

	tool := MustTool("test_tool", "A test tool", testToolHandler)
	srv := server.NewMCPServer("test", "0")
	tool.Register(srv)

	var schema map[string]any
	require.NoError(t, json.Unmarshal(tool.Tool.RawInputSchema, &schema))
	props := schema["properties"].(map[string]any)
	_, hasEndpoint := props["endpoint"]
	assert.False(t, hasEndpoint, "without an instance store, Register must not inject endpoint")
}

// TestInjectEndpointIntoSchemaPreservesOtherFields exercises the helper
// directly — checks $defs, required, additionalProperties round-trip cleanly.
func TestInjectEndpointIntoSchemaPreservesOtherFields(t *testing.T) {
	original := []byte(`{
		"type":"object",
		"properties":{"name":{"type":"string"}},
		"required":["name"],
		"additionalProperties":false
	}`)

	out, err := injectEndpointIntoSchema(original, []string{"a", "b"})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "object", got["type"])
	assert.Equal(t, false, got["additionalProperties"])
	assert.Equal(t, []any{"name"}, got["required"])

	props := got["properties"].(map[string]any)
	_, hasName := props["name"]
	assert.True(t, hasName, "original 'name' property should survive")
	_, hasEndpoint := props["endpoint"]
	assert.True(t, hasEndpoint, "endpoint should be injected")
}
