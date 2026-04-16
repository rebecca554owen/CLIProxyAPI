package claude

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertClaudeRequestToGemini_ToolChoice_SpecificTool(t *testing.T) {
	inputJSON := []byte(`{
		"model": "gemini-3-flash-preview",
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "hi"}
				]
			}
		],
		"tools": [
			{
				"name": "json",
				"description": "A JSON tool",
				"input_schema": {
					"type": "object",
					"properties": {}
				}
			}
		],
		"tool_choice": {"type": "tool", "name": "json"}
	}`)

	output := ConvertClaudeRequestToGemini("gemini-3-flash-preview", inputJSON, false)

	if got := gjson.GetBytes(output, "toolConfig.functionCallingConfig.mode").String(); got != "ANY" {
		t.Fatalf("Expected toolConfig.functionCallingConfig.mode 'ANY', got '%s'", got)
	}
	allowed := gjson.GetBytes(output, "toolConfig.functionCallingConfig.allowedFunctionNames").Array()
	if len(allowed) != 1 || allowed[0].String() != "json" {
		t.Fatalf("Expected allowedFunctionNames ['json'], got %s", gjson.GetBytes(output, "toolConfig.functionCallingConfig.allowedFunctionNames").Raw)
	}
}

func TestConvertClaudeRequestToGemini_ImageContent(t *testing.T) {
	inputJSON := []byte(`{
		"model": "gemini-3-flash-preview",
		"messages": [
			{
				"role": "user",
				"content": [
					{"type": "text", "text": "describe this image"},
					{
						"type": "image",
						"source": {
							"type": "base64",
							"media_type": "image/png",
							"data": "aGVsbG8="
						}
					}
				]
			}
		]
	}`)

	output := ConvertClaudeRequestToGemini("gemini-3-flash-preview", inputJSON, false)

	parts := gjson.GetBytes(output, "contents.0.parts").Array()
	if len(parts) != 2 {
		t.Fatalf("Expected 2 parts, got %d", len(parts))
	}
	if got := parts[0].Get("text").String(); got != "describe this image" {
		t.Fatalf("Expected first part text 'describe this image', got '%s'", got)
	}
	if got := parts[1].Get("inline_data.mime_type").String(); got != "image/png" {
		t.Fatalf("Expected image mime type 'image/png', got '%s'", got)
	}
	if got := parts[1].Get("inline_data.data").String(); got != "aGVsbG8=" {
		t.Fatalf("Expected image data 'aGVsbG8=', got '%s'", got)
	}
}

func TestConvertClaudeRequestToGemini_NormalizesPollutedInteropTranscript(t *testing.T) {
	inputJSON := []byte(`{
		"model": "gemini-3-flash-preview",
		"messages": [
			{
				"role": "assistant",
				"content": [{"type": "input_text", "text": "pre"}],
				"tool_calls": [
					{"id": "call_1", "type": "function", "function": {"name": "sessions_list", "arguments": "{\"limit\":10}"}}
				]
			},
			{
				"role": "tool",
				"tool_call_id": "call_1",
				"content": "ok"
			}
		]
	}`)

	output := ConvertClaudeRequestToGemini("gemini-3-flash-preview", inputJSON, false)
	contents := gjson.GetBytes(output, "contents").Array()

	if len(contents) != 2 {
		t.Fatalf("Expected 2 contents, got %d: %s", len(contents), gjson.GetBytes(output, "contents").Raw)
	}
	if got := contents[0].Get("role").String(); got != "model" {
		t.Fatalf("Expected first role model, got %q", got)
	}
	if got := contents[0].Get("parts.0.text").String(); got != "pre" {
		t.Fatalf("Expected first part text %q, got %q", "pre", got)
	}
	if got := contents[0].Get("parts.1.functionCall.name").String(); got != "sessions_list" {
		t.Fatalf("Expected functionCall name %q, got %q", "sessions_list", got)
	}
	if got := contents[1].Get("role").String(); got != "user" {
		t.Fatalf("Expected second role user, got %q", got)
	}
	if got := contents[1].Get("parts.0.functionResponse.name").String(); got != "call_1" {
		t.Fatalf("Expected functionResponse name %q, got %q", "call_1", got)
	}
}
