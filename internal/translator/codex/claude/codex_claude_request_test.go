package claude

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertClaudeRequestToCodex_SystemMessageScenarios(t *testing.T) {
	tests := []struct {
		name             string
		inputJSON        string
		wantHasDeveloper bool
		wantTexts        []string
	}{
		{
			name: "No system field",
			inputJSON: `{
				"model": "claude-3-opus",
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantHasDeveloper: false,
		},
		{
			name: "Empty string system field",
			inputJSON: `{
				"model": "claude-3-opus",
				"system": "",
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantHasDeveloper: false,
		},
		{
			name: "String system field",
			inputJSON: `{
				"model": "claude-3-opus",
				"system": "Be helpful",
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantHasDeveloper: true,
			wantTexts:        []string{"Be helpful"},
		},
		{
			name: "Array system field with filtered billing header",
			inputJSON: `{
				"model": "claude-3-opus",
				"system": [
					{"type": "text", "text": "x-anthropic-billing-header: tenant-123"},
					{"type": "text", "text": "Block 1"},
					{"type": "text", "text": "Block 2"}
				],
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantHasDeveloper: true,
			wantTexts:        []string{"Block 1", "Block 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertClaudeRequestToCodex("test-model", []byte(tt.inputJSON), false)
			resultJSON := gjson.ParseBytes(result)
			inputs := resultJSON.Get("input").Array()

			hasDeveloper := len(inputs) > 0 && inputs[0].Get("role").String() == "developer"
			if hasDeveloper != tt.wantHasDeveloper {
				t.Fatalf("got hasDeveloper = %v, want %v. Output: %s", hasDeveloper, tt.wantHasDeveloper, resultJSON.Get("input").Raw)
			}

			if !tt.wantHasDeveloper {
				return
			}

			content := inputs[0].Get("content").Array()
			if len(content) != len(tt.wantTexts) {
				t.Fatalf("got %d system content items, want %d. Content: %s", len(content), len(tt.wantTexts), inputs[0].Get("content").Raw)
			}

			for i, wantText := range tt.wantTexts {
				if gotType := content[i].Get("type").String(); gotType != "input_text" {
					t.Fatalf("content[%d] type = %q, want %q", i, gotType, "input_text")
				}
				if gotText := content[i].Get("text").String(); gotText != wantText {
					t.Fatalf("content[%d] text = %q, want %q", i, gotText, wantText)
				}
			}
		})
	}
}

func TestConvertClaudeRequestToCodex_ParallelToolCalls(t *testing.T) {
	tests := []struct {
		name                  string
		inputJSON             string
		wantParallelToolCalls bool
	}{
		{
			name: "Default to true when tool_choice.disable_parallel_tool_use is absent",
			inputJSON: `{
				"model": "claude-3-opus",
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantParallelToolCalls: true,
		},
		{
			name: "Disable parallel tool calls when client opts out",
			inputJSON: `{
				"model": "claude-3-opus",
				"tool_choice": {"disable_parallel_tool_use": true},
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantParallelToolCalls: false,
		},
		{
			name: "Keep parallel tool calls enabled when client explicitly allows them",
			inputJSON: `{
				"model": "claude-3-opus",
				"tool_choice": {"disable_parallel_tool_use": false},
				"messages": [{"role": "user", "content": "hello"}]
			}`,
			wantParallelToolCalls: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertClaudeRequestToCodex("test-model", []byte(tt.inputJSON), false)
			resultJSON := gjson.ParseBytes(result)

			if got := resultJSON.Get("parallel_tool_calls").Bool(); got != tt.wantParallelToolCalls {
				t.Fatalf("parallel_tool_calls = %v, want %v. Output: %s", got, tt.wantParallelToolCalls, string(result))
			}
		})
	}
}

func TestConvertClaudeRequestToCodex_NormalizesPollutedInteropTranscript(t *testing.T) {
	inputJSON := `{
		"model": "claude-3-opus",
		"messages": [
			{
				"role": "assistant",
				"content": [
					{"type": "input_text", "text": "pre"},
					{"type": "thinking", "thinking": "internal"}
				],
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
	}`

	result := ConvertClaudeRequestToCodex("test-model", []byte(inputJSON), false)
	items := gjson.GetBytes(result, "input").Array()

	if len(items) != 3 {
		t.Fatalf("expected 3 input items, got %d: %s", len(items), gjson.GetBytes(result, "input").Raw)
	}
	if got := items[0].Get("type").String(); got != "message" {
		t.Fatalf("expected first item message, got %q", got)
	}
	if got := items[0].Get("content.0.type").String(); got != "output_text" {
		t.Fatalf("expected first message content type %q, got %q", "output_text", got)
	}
	if got := items[0].Get("reasoning_content").String(); got != "internal" {
		t.Fatalf("expected reasoning_content %q, got %q", "internal", got)
	}
	if got := items[1].Get("type").String(); got != "function_call" {
		t.Fatalf("expected second item function_call, got %q", got)
	}
	if got := items[2].Get("type").String(); got != "function_call_output" {
		t.Fatalf("expected third item function_call_output, got %q", got)
	}
	if got := items[2].Get("output").String(); got != "ok" {
		t.Fatalf("expected tool output %q, got %q", "ok", got)
	}
}
