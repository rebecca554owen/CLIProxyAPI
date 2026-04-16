package util

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func NormalizeOpenAIResponsesRequestJSON(input []byte) []byte {
	if len(input) == 0 || !gjson.ValidBytes(input) {
		return input
	}
	root := gjson.ParseBytes(input)
	in := root.Get("input")
	if !in.Exists() || !in.IsArray() {
		return input
	}

	normalized := normalizeResponsesInputArray(in.Array())
	if normalized == "" || normalized == in.Raw {
		return input
	}
	out, err := sjson.SetRawBytes(input, "input", []byte(normalized))
	if err != nil {
		return input
	}
	return out
}

func NormalizeOpenAIChatRequestJSON(input []byte) []byte {
	if len(input) == 0 || !gjson.ValidBytes(input) {
		return input
	}
	root := gjson.ParseBytes(input)
	msgs := root.Get("messages")
	if !msgs.Exists() || !msgs.IsArray() {
		return input
	}

	normalized := normalizeChatMessagesArray(msgs.Array())
	if normalized == "" || normalized == msgs.Raw {
		return input
	}
	out, err := sjson.SetRawBytes(input, "messages", []byte(normalized))
	if err != nil {
		return input
	}
	return out
}

func NormalizeClaudeRequestJSON(input []byte) []byte {
	if len(input) == 0 || !gjson.ValidBytes(input) {
		return input
	}

	out := input
	root := gjson.ParseBytes(out)

	if system := root.Get("system"); system.Exists() {
		normalizedSystem := normalizeClaudeSystem(system)
		if normalizedSystem != "" && normalizedSystem != system.Raw {
			next, err := sjson.SetRawBytes(out, "system", []byte(normalizedSystem))
			if err == nil {
				out = next
				root = gjson.ParseBytes(out)
			}
		}
	}

	messages := root.Get("messages")
	if !messages.Exists() || !messages.IsArray() {
		return out
	}

	normalizedMessages := normalizeClaudeMessagesArray(messages.Array())
	if normalizedMessages == "" || normalizedMessages == messages.Raw {
		return out
	}
	next, err := sjson.SetRawBytes(out, "messages", []byte(normalizedMessages))
	if err != nil {
		return out
	}
	return next
}

func normalizeResponsesInputArray(items []gjson.Result) string {
	out := []byte(`[]`)
	changed := false

	for _, item := range items {
		itemType := item.Get("type").String()
		itemRole := item.Get("role").String()
		if itemType == "" && itemRole != "" {
			itemType = "message"
		}

		switch itemType {
		case "message":
			msgRaw, extra := normalizeResponsesMessageItem(item)
			if msgRaw != "" {
				out, _ = sjson.SetRawBytes(out, "-1", []byte(msgRaw))
				if msgRaw != item.Raw {
					changed = true
				}
			}
			for _, extraItem := range extra {
				out, _ = sjson.SetRawBytes(out, "-1", []byte(extraItem))
				changed = true
			}
		case "tool_use":
			call := buildResponsesFunctionCall(
				strings.TrimSpace(item.Get("id").String()),
				strings.TrimSpace(item.Get("name").String()),
				jsonValueToString(item.Get("input").Value(), "{}"),
			)
			out, _ = sjson.SetRawBytes(out, "-1", []byte(call))
			changed = true
		case "tool_result":
			result := buildResponsesFunctionCallOutput(
				strings.TrimSpace(item.Get("tool_use_id").String()),
				toolResultValue(item.Get("content")),
			)
			out, _ = sjson.SetRawBytes(out, "-1", []byte(result))
			changed = true
		default:
			out, _ = sjson.SetRawBytes(out, "-1", []byte(item.Raw))
		}
	}

	if !changed {
		return ""
	}
	return string(out)
}

func normalizeResponsesMessageItem(item gjson.Result) (string, []string) {
	msg := []byte(`{}`)
	msg, _ = sjson.SetBytes(msg, "type", "message")
	role := strings.TrimSpace(item.Get("role").String())
	if role == "" {
		role = "user"
	}
	msg, _ = sjson.SetBytes(msg, "role", role)

	content := item.Get("content")
	extra := make([]string, 0)
	reasoning := strings.TrimSpace(item.Get("reasoning_content").String())
	contentAdded := false
	if content.IsArray() {
		for _, part := range content.Array() {
			partType := strings.TrimSpace(part.Get("type").String())
			switch partType {
			case "input_text", "output_text", "input_image", "input_audio", "input_file":
				msg, _ = sjson.SetRawBytes(msg, "content.-1", []byte(part.Raw))
				contentAdded = true
			case "text":
				normalizedType := "input_text"
				if role == "assistant" || role == "model" {
					normalizedType = "output_text"
				}
				textPart := []byte(`{}`)
				textPart, _ = sjson.SetBytes(textPart, "type", normalizedType)
				textPart, _ = sjson.SetBytes(textPart, "text", part.Get("text").String())
				msg, _ = sjson.SetRawBytes(msg, "content.-1", textPart)
				contentAdded = true
			case "image":
				if dataURL := claudeImageSourceToDataURL(part.Get("source")); dataURL != "" {
					imagePart := []byte(`{}`)
					imagePart, _ = sjson.SetBytes(imagePart, "type", "input_image")
					imagePart, _ = sjson.SetBytes(imagePart, "image_url", dataURL)
					msg, _ = sjson.SetRawBytes(msg, "content.-1", imagePart)
					contentAdded = true
				}
			case "tool_use":
				callID := strings.TrimSpace(part.Get("id").String())
				name := strings.TrimSpace(part.Get("name").String())
				args := jsonValueToString(part.Get("input").Value(), "{}")
				extra = append(extra, buildResponsesFunctionCall(callID, name, args))
			case "tool_result":
				callID := strings.TrimSpace(part.Get("tool_use_id").String())
				output := toolResultValue(part.Get("content"))
				extra = append(extra, buildResponsesFunctionCallOutput(callID, output))
			case "thinking":
				if reasoning == "" {
					reasoning = strings.TrimSpace(part.Get("thinking").String())
				}
			}
		}
	} else if content.Exists() && content.Type == gjson.String {
		textPart := []byte(`{}`)
		partType := "input_text"
		if role == "assistant" || role == "model" {
			partType = "output_text"
		}
		textPart, _ = sjson.SetBytes(textPart, "type", partType)
		textPart, _ = sjson.SetBytes(textPart, "text", content.String())
		msg, _ = sjson.SetRawBytes(msg, "content.-1", textPart)
		contentAdded = true
	}

	if tc := item.Get("tool_calls"); tc.Exists() && tc.IsArray() {
		for _, call := range tc.Array() {
			if call.Get("type").String() != "function" {
				continue
			}
			callID := strings.TrimSpace(call.Get("id").String())
			name := strings.TrimSpace(call.Get("function.name").String())
			args := call.Get("function.arguments").String()
			extra = append(extra, buildResponsesFunctionCall(callID, name, args))
		}
	}

	if reasoning != "" {
		msg, _ = sjson.SetBytes(msg, "reasoning_content", reasoning)
	}
	if !contentAdded {
		msg, _ = sjson.SetRawBytes(msg, "content", []byte(`[]`))
	}
	return string(msg), extra
}

func normalizeChatMessagesArray(messages []gjson.Result) string {
	out := []byte(`[]`)
	changed := false

	for _, message := range messages {
		msg, extra := normalizeChatMessage(message)
		if msg != "" {
			out, _ = sjson.SetRawBytes(out, "-1", []byte(msg))
			if msg != message.Raw {
				changed = true
			}
		}
		for _, extraMsg := range extra {
			out, _ = sjson.SetRawBytes(out, "-1", []byte(extraMsg))
			changed = true
		}
	}

	if !changed {
		return ""
	}
	return string(out)
}

func normalizeClaudeSystem(system gjson.Result) string {
	switch {
	case system.Type == gjson.String:
		text := system.String()
		if strings.TrimSpace(text) == "" {
			return ""
		}
		block := []byte(`[]`)
		textPart := []byte(`{"type":"text","text":""}`)
		textPart, _ = sjson.SetBytes(textPart, "text", text)
		block, _ = sjson.SetRawBytes(block, "-1", textPart)
		return string(block)
	case system.IsArray():
		out := []byte(`[]`)
		changed := false
		for _, part := range system.Array() {
			normalized, partChanged := normalizeClaudeContentPart(part)
			if normalized == "" {
				changed = true
				continue
			}
			out, _ = sjson.SetRawBytes(out, "-1", []byte(normalized))
			if partChanged {
				changed = true
			}
		}
		if !changed {
			return ""
		}
		return string(out)
	default:
		return ""
	}
}

func normalizeClaudeMessagesArray(messages []gjson.Result) string {
	out := []byte(`[]`)
	changed := false

	for _, message := range messages {
		items, itemChanged := normalizeClaudeMessage(message)
		for _, item := range items {
			out, _ = sjson.SetRawBytes(out, "-1", []byte(item))
		}
		if itemChanged {
			changed = true
		}
	}

	if !changed {
		return ""
	}
	return string(out)
}

func normalizeClaudeMessage(message gjson.Result) ([]string, bool) {
	role := strings.TrimSpace(message.Get("role").String())
	msg := []byte(message.Raw)
	changed := false

	if role == "tool" {
		toolResult := []byte(`{"type":"tool_result","tool_use_id":"","content":""}`)
		toolResult, _ = sjson.SetBytes(toolResult, "tool_use_id", message.Get("tool_call_id").String())
		if content := message.Get("content"); content.Exists() {
			if content.Type == gjson.String {
				toolResult, _ = sjson.SetBytes(toolResult, "content", content.String())
			} else {
				toolResult, _ = sjson.SetRawBytes(toolResult, "content", []byte(content.Raw))
			}
		}
		userMessage := []byte(`{"role":"user","content":[]}`)
		userMessage, _ = sjson.SetRawBytes(userMessage, "content.-1", toolResult)
		return []string{string(userMessage)}, true
	}

	content := message.Get("content")
	normalizedContent := []byte(`[]`)
	contentChanged := false
	reasoning := strings.TrimSpace(message.Get("reasoning_content").String())
	contentExists := false

	if content.Type == gjson.String {
		if text := content.String(); strings.TrimSpace(text) != "" {
			part := []byte(`{"type":"text","text":""}`)
			part, _ = sjson.SetBytes(part, "text", text)
			normalizedContent, _ = sjson.SetRawBytes(normalizedContent, "-1", part)
			contentExists = true
			contentChanged = true
		}
	} else if content.IsArray() {
		for _, part := range content.Array() {
			partType := strings.TrimSpace(part.Get("type").String())
			switch partType {
			case "thinking":
				if role == "assistant" {
					text := strings.TrimSpace(part.Get("thinking").String())
					if text != "" {
						if reasoning == "" {
							reasoning = text
						} else {
							reasoning += "\n\n" + text
						}
					}
				}
				contentChanged = true
			case "redacted_thinking":
				contentChanged = true
			default:
				normalized, partChanged := normalizeClaudeContentPart(part)
				if normalized == "" {
					if partChanged {
						contentChanged = true
					}
					continue
				}
				normalizedContent, _ = sjson.SetRawBytes(normalizedContent, "-1", []byte(normalized))
				contentExists = true
				if partChanged {
					contentChanged = true
				}
			}
		}
	}

	if calls := message.Get("tool_calls"); calls.Exists() && calls.IsArray() {
		for _, call := range calls.Array() {
			if call.Get("type").String() != "function" {
				continue
			}
			part := []byte(`{"type":"tool_use","id":"","name":"","input":{}}`)
			part, _ = sjson.SetBytes(part, "id", call.Get("id").String())
			part, _ = sjson.SetBytes(part, "name", call.Get("function.name").String())
			args := strings.TrimSpace(call.Get("function.arguments").String())
			if args != "" && gjson.Valid(args) && gjson.Parse(args).IsObject() {
				part, _ = sjson.SetRawBytes(part, "input", []byte(args))
			}
			normalizedContent, _ = sjson.SetRawBytes(normalizedContent, "-1", part)
			contentExists = true
			contentChanged = true
		}
	}

	if !contentChanged && strings.TrimSpace(message.Get("reasoning_content").String()) == reasoning {
		return []string{message.Raw}, false
	}

	if contentExists {
		msg, _ = sjson.SetRawBytes(msg, "content", normalizedContent)
	} else {
		msg, _ = sjson.SetRawBytes(msg, "content", []byte(`[]`))
	}
	if reasoning != "" {
		msg, _ = sjson.SetBytes(msg, "reasoning_content", reasoning)
	} else {
		msg, _ = sjson.DeleteBytes(msg, "reasoning_content")
	}

	changed = contentChanged || reasoning != strings.TrimSpace(message.Get("reasoning_content").String())
	return []string{string(msg)}, changed
}

func normalizeClaudeContentPart(part gjson.Result) (string, bool) {
	partType := strings.TrimSpace(part.Get("type").String())
	switch partType {
	case "text", "image", "tool_use", "tool_result":
		return part.Raw, false
	case "input_text", "output_text":
		item := []byte(`{"type":"text","text":""}`)
		item, _ = sjson.SetBytes(item, "text", part.Get("text").String())
		return string(item), true
	case "image_url":
		url := strings.TrimSpace(part.Get("image_url.url").String())
		if url == "" {
			url = strings.TrimSpace(part.Get("image_url").String())
		}
		if url == "" {
			return "", true
		}
		item := []byte(`{"type":"image","source":{"type":"url","url":""}}`)
		item, _ = sjson.SetBytes(item, "source.url", url)
		return string(item), true
	case "input_image":
		url := strings.TrimSpace(part.Get("image_url").String())
		if url == "" {
			return "", true
		}
		item := []byte(`{"type":"image","source":{"type":"url","url":""}}`)
		item, _ = sjson.SetBytes(item, "source.url", url)
		return string(item), true
	case "function_call":
		item := []byte(`{"type":"tool_use","id":"","name":"","input":{}}`)
		item, _ = sjson.SetBytes(item, "id", part.Get("call_id").String())
		item, _ = sjson.SetBytes(item, "name", part.Get("name").String())
		args := strings.TrimSpace(part.Get("arguments").String())
		if args != "" && gjson.Valid(args) && gjson.Parse(args).IsObject() {
			item, _ = sjson.SetRawBytes(item, "input", []byte(args))
		}
		return string(item), true
	case "function_call_output":
		item := []byte(`{"type":"tool_result","tool_use_id":"","content":""}`)
		item, _ = sjson.SetBytes(item, "tool_use_id", part.Get("call_id").String())
		output := part.Get("output")
		if output.Exists() {
			if output.Type == gjson.String {
				item, _ = sjson.SetBytes(item, "content", output.String())
			} else {
				item, _ = sjson.SetRawBytes(item, "content", []byte(output.Raw))
			}
		}
		return string(item), true
	default:
		return part.Raw, false
	}
}

func normalizeChatMessage(message gjson.Result) (string, []string) {
	msg := []byte(message.Raw)
	role := strings.TrimSpace(message.Get("role").String())
	content := message.Get("content")
	if !content.IsArray() {
		return string(msg), nil
	}

	normalizedContent := []byte(`[]`)
	extra := make([]string, 0)
	contentChanged := false
	reasoning := strings.TrimSpace(message.Get("reasoning_content").String())
	toolCalls := message.Get("tool_calls").Raw
	hasToolCalls := message.Get("tool_calls").IsArray()

	for _, part := range content.Array() {
		partType := strings.TrimSpace(part.Get("type").String())
		switch partType {
		case "text", "image_url", "file":
			normalizedContent, _ = sjson.SetRawBytes(normalizedContent, "-1", []byte(part.Raw))
		case "input_text", "output_text":
			textPart := []byte(`{"type":"text","text":""}`)
			textPart, _ = sjson.SetBytes(textPart, "text", part.Get("text").String())
			normalizedContent, _ = sjson.SetRawBytes(normalizedContent, "-1", textPart)
			contentChanged = true
		case "tool_use":
			call := []byte(`{"id":"","type":"function","function":{"name":"","arguments":""}}`)
			call, _ = sjson.SetBytes(call, "id", part.Get("id").String())
			call, _ = sjson.SetBytes(call, "function.name", part.Get("name").String())
			call, _ = sjson.SetBytes(call, "function.arguments", jsonValueToString(part.Get("input").Value(), "{}"))
			if !hasToolCalls {
				toolCalls = `[]`
				hasToolCalls = true
			}
			toolCallsBytes, _ := sjson.SetRawBytes([]byte(toolCalls), "-1", call)
			toolCalls = string(toolCallsBytes)
			contentChanged = true
		case "tool_result":
			toolMsg := []byte(`{"role":"tool","tool_call_id":"","content":""}`)
			toolMsg, _ = sjson.SetBytes(toolMsg, "tool_call_id", part.Get("tool_use_id").String())
			toolMsg, _ = sjson.SetBytes(toolMsg, "content", toolResultValue(part.Get("content")))
			extra = append(extra, string(toolMsg))
			contentChanged = true
		case "thinking":
			if role == "assistant" && reasoning == "" {
				reasoning = strings.TrimSpace(part.Get("thinking").String())
			}
			contentChanged = true
		default:
			normalizedContent, _ = sjson.SetRawBytes(normalizedContent, "-1", []byte(part.Raw))
		}
	}

	if !contentChanged {
		return string(msg), nil
	}
	msg, _ = sjson.SetRawBytes(msg, "content", normalizedContent)
	if hasToolCalls {
		msg, _ = sjson.SetRawBytes(msg, "tool_calls", []byte(toolCalls))
	}
	if reasoning != "" {
		msg, _ = sjson.SetBytes(msg, "reasoning_content", reasoning)
	}
	return string(msg), extra
}

func buildResponsesFunctionCall(callID, name, args string) string {
	item := []byte(`{"type":"function_call","call_id":"","name":"","arguments":"{}"}`)
	item, _ = sjson.SetBytes(item, "call_id", callID)
	item, _ = sjson.SetBytes(item, "name", name)
	if strings.TrimSpace(args) == "" {
		args = "{}"
	}
	item, _ = sjson.SetBytes(item, "arguments", args)
	return string(item)
}

func buildResponsesFunctionCallOutput(callID, output string) string {
	item := []byte(`{"type":"function_call_output","call_id":"","output":""}`)
	item, _ = sjson.SetBytes(item, "call_id", callID)
	item, _ = sjson.SetBytes(item, "output", output)
	return string(item)
}

func jsonValueToString(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return fallback
		}
		return typed
	default:
		raw, err := json.Marshal(value)
		if err != nil || len(raw) == 0 {
			return fallback
		}
		return string(raw)
	}
}

func toolResultValue(content gjson.Result) string {
	if !content.Exists() {
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if content.IsArray() {
		parts := make([]string, 0, len(content.Array()))
		for _, item := range content.Array() {
			switch item.Get("type").String() {
			case "text":
				if text := strings.TrimSpace(item.Get("text").String()); text != "" {
					parts = append(parts, text)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return content.Raw
}

func claudeImageSourceToDataURL(source gjson.Result) string {
	if !source.Exists() {
		return ""
	}
	switch source.Get("type").String() {
	case "base64":
		mediaType := strings.TrimSpace(source.Get("media_type").String())
		data := strings.TrimSpace(source.Get("data").String())
		if mediaType == "" || data == "" {
			return ""
		}
		return "data:" + mediaType + ";base64," + data
	case "url":
		return strings.TrimSpace(source.Get("url").String())
	default:
		return ""
	}
}
