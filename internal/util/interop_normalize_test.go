package util

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAIResponsesRequestJSON_ConvertsClaudeBlocks(t *testing.T) {
	input := []byte(`{
		"input":[
			{
				"role":"assistant",
				"content":[
					{"type":"text","text":"checking"},
					{"type":"tool_use","id":"call_1","name":"sessions_list","input":{"limit":10}}
				]
			},
			{
				"role":"user",
				"content":[
					{"type":"tool_result","tool_use_id":"call_1","content":"ok"}
				]
			}
		]
	}`)

	out := NormalizeOpenAIResponsesRequestJSON(input)
	items := gjson.GetBytes(out, "input").Array()
	if len(items) != 3 {
		t.Fatalf("expected 3 normalized items, got %d: %s", len(items), gjson.GetBytes(out, "input").Raw)
	}
	if items[1].Get("type").String() != "function_call" {
		t.Fatalf("expected item 1 function_call, got %s", items[1].Raw)
	}
	if items[2].Get("type").String() != "function_call_output" {
		t.Fatalf("expected function_call_output tail: %s", gjson.GetBytes(out, "input").Raw)
	}
}

func TestNormalizeOpenAIResponsesRequestJSON_PreservesMixedContentOrder(t *testing.T) {
	input := []byte(`{
		"input":[
			{
				"role":"assistant",
				"reasoning_content":"internal",
				"content":[
					{"type":"tool_use","id":"call_1","name":"sessions_list","input":{"limit":10}},
					{"type":"text","text":"after tool"}
				]
			},
			{
				"role":"assistant",
				"content":[
					{"type":"text","text":"before"},
					{"type":"tool_use","id":"call_2","name":"sessions_get","input":{"id":"1"}},
					{"type":"text","text":"after"}
				]
			}
		]
	}`)

	out := NormalizeOpenAIResponsesRequestJSON(input)
	items := gjson.GetBytes(out, "input").Array()
	if len(items) != 5 {
		t.Fatalf("expected 5 normalized items, got %d: %s", len(items), gjson.GetBytes(out, "input").Raw)
	}
	if got := items[0].Get("type").String(); got != "function_call" {
		t.Fatalf("expected item 0 function_call, got %s", items[0].Raw)
	}
	if got := items[1].Get("type").String(); got != "message" {
		t.Fatalf("expected item 1 message, got %s", items[1].Raw)
	}
	if got := items[1].Get("content.0.text").String(); got != "after tool" {
		t.Fatalf("expected item 1 text after tool, got %s", items[1].Raw)
	}
	if got := items[1].Get("reasoning_content").String(); got != "internal" {
		t.Fatalf("expected reasoning on first emitted assistant message, got %s", items[1].Raw)
	}
	if got := items[2].Get("content.0.text").String(); got != "before" {
		t.Fatalf("expected item 2 text before tool, got %s", items[2].Raw)
	}
	if got := items[3].Get("type").String(); got != "function_call" {
		t.Fatalf("expected item 3 function_call, got %s", items[3].Raw)
	}
	if got := items[4].Get("content.0.text").String(); got != "after" {
		t.Fatalf("expected item 4 text after tool, got %s", items[4].Raw)
	}
}

func TestNormalizeOpenAIResponsesRequestJSON_PreservesStructuredToolResultOutput(t *testing.T) {
	input := []byte(`{
		"input":[
			{
				"role":"user",
				"content":[
					{"type":"tool_result","tool_use_id":"call_obj","content":{"ok":true,"items":[1,2]}},
					{"type":"tool_result","tool_use_id":"call_arr","content":[{"type":"json","value":{"nested":1}}]},
					{"type":"tool_result","tool_use_id":"call_num","content":123},
					{"type":"tool_result","tool_use_id":"call_bool","content":false},
					{"type":"tool_result","tool_use_id":"call_null","content":null}
				]
			}
		]
	}`)

	out := NormalizeOpenAIResponsesRequestJSON(input)
	items := gjson.GetBytes(out, "input").Array()
	if len(items) != 5 {
		t.Fatalf("expected 5 normalized items, got %d: %s", len(items), gjson.GetBytes(out, "input").Raw)
	}
	if !items[0].Get("output").IsObject() {
		t.Fatalf("expected object output, got %s", items[0].Raw)
	}
	if !items[1].Get("output").IsArray() {
		t.Fatalf("expected array output, got %s", items[1].Raw)
	}
	if got := items[2].Get("output").Raw; got != "123" {
		t.Fatalf("expected numeric output 123, got %s", items[2].Raw)
	}
	if got := items[3].Get("output").Raw; got != "false" {
		t.Fatalf("expected bool output false, got %s", items[3].Raw)
	}
	if got := items[4].Get("output").Raw; got != "null" {
		t.Fatalf("expected null output, got %s", items[4].Raw)
	}
}

func TestNormalizeOpenAIChatRequestJSON_ConvertsClaudeBlocks(t *testing.T) {
	input := []byte(`{
		"messages":[
			{
				"role":"assistant",
				"content":[
					{"type":"text","text":"checking"},
					{"type":"tool_use","id":"call_1","name":"sessions_list","input":{"limit":10}},
					{"type":"thinking","thinking":"internal"}
				]
			},
			{
				"role":"user",
				"content":[
					{"type":"tool_result","tool_use_id":"call_1","content":"ok"}
				]
			}
		]
	}`)

	out := NormalizeOpenAIChatRequestJSON(input)
	msgs := gjson.GetBytes(out, "messages").Array()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 normalized messages, got %d: %s", len(msgs), gjson.GetBytes(out, "messages").Raw)
	}
	if !msgs[0].Get("tool_calls").IsArray() {
		t.Fatalf("assistant tool_calls should be synthesized: %s", msgs[0].Raw)
	}
	if got := msgs[0].Get("reasoning_content").String(); got != "internal" {
		t.Fatalf("expected reasoning_content=internal, got %q", got)
	}
	if got := msgs[2].Get("role").String(); got != "tool" {
		t.Fatalf("expected appended tool role, got %q: %s", got, msgs[2].Raw)
	}
}

func TestNormalizeOpenAIChatRequestJSON_PreservesStructuredToolMessageContent(t *testing.T) {
	input := []byte(`{
		"messages":[
			{
				"role":"user",
				"content":[
					{"type":"tool_result","tool_use_id":"call_1","content":{"ok":true,"items":[1,2]}}
				]
			}
		]
	}`)

	out := NormalizeOpenAIChatRequestJSON(input)
	msgs := gjson.GetBytes(out, "messages").Array()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 normalized messages, got %d: %s", len(msgs), gjson.GetBytes(out, "messages").Raw)
	}
	if got := msgs[1].Get("role").String(); got != "tool" {
		t.Fatalf("expected appended tool message, got %s", msgs[1].Raw)
	}
	if !msgs[1].Get("content").IsObject() {
		t.Fatalf("expected structured tool content object, got %s", msgs[1].Raw)
	}
}

func TestNormalizeClaudeRequestJSON_ConvertsPollutedInteropMessages(t *testing.T) {
	input := []byte(`{
		"system":"Be precise",
		"messages":[
			{
				"role":"assistant",
				"content":[
					{"type":"input_text","text":"draft"},
					{"type":"thinking","thinking":"internal"}
				],
				"tool_calls":[
					{"id":"call_1","type":"function","function":{"name":"sessions_list","arguments":"{\"limit\":10}"}}
				]
			},
			{
				"role":"tool",
				"tool_call_id":"call_1",
				"content":"ok"
			}
		]
	}`)

	out := NormalizeClaudeRequestJSON(input)

	if got := gjson.GetBytes(out, "system.0.type").String(); got != "text" {
		t.Fatalf("expected normalized system text block, got %s", gjson.GetBytes(out, "system").Raw)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.type").String(); got != "text" {
		t.Fatalf("expected assistant content[0] text, got %s", gjson.GetBytes(out, "messages.0.content").Raw)
	}
	if got := gjson.GetBytes(out, "messages.0.reasoning_content").String(); got != "internal" {
		t.Fatalf("expected reasoning_content=internal, got %q", got)
	}
	if got := gjson.GetBytes(out, "messages.0.content.1.type").String(); got != "tool_use" {
		t.Fatalf("expected synthesized tool_use, got %s", gjson.GetBytes(out, "messages.0.content").Raw)
	}
	if got := gjson.GetBytes(out, "messages.1.role").String(); got != "user" {
		t.Fatalf("expected tool role to normalize into user role, got %q", got)
	}
	if got := gjson.GetBytes(out, "messages.1.content.0.type").String(); got != "tool_result" {
		t.Fatalf("expected normalized tool_result, got %s", gjson.GetBytes(out, "messages.1.content").Raw)
	}
}
