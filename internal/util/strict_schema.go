package util

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
)

const strictObjectJSONSchema = `{"type":"object","properties":{},"additionalProperties":false}`

func CleanJSONSchemaForStrictUpstream(jsonStr string) string {
	return cleanJSONSchemaForStrictUpstream(jsonStr, false)
}

func CleanJSONSchemaForOpenAIStructuredOutput(jsonStr string) string {
	return cleanJSONSchemaForStrictUpstream(jsonStr, true)
}

func cleanJSONSchemaForStrictUpstream(jsonStr string, requireAllObjectProperties bool) string {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" || jsonStr == "null" || !gjson.Valid(jsonStr) {
		return strictObjectJSONSchema
	}

	jsonStr = CleanJSONSchemaForGemini(jsonStr)
	if !gjson.Valid(jsonStr) {
		return strictObjectJSONSchema
	}

	var parsed any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return strictObjectJSONSchema
	}

	root, ok := normalizeStrictSchemaNode(parsed).(map[string]any)
	if !ok {
		return strictObjectJSONSchema
	}

	if schemaType, ok := root["type"].(string); !ok || strings.TrimSpace(schemaType) == "" {
		root["type"] = "object"
	}
	if root["type"] == "object" {
		if _, ok := root["properties"].(map[string]any); !ok {
			root["properties"] = map[string]any{}
		}
		root["additionalProperties"] = false
	}
	if requireAllObjectProperties {
		requireAllPropertiesForObjects(root)
	} else {
		if req := normalizeStringArray(root["required"]); len(req) > 0 {
			root["required"] = req
		} else {
			delete(root, "required")
		}
	}

	out, err := json.Marshal(root)
	if err != nil || !gjson.ValidBytes(out) {
		return strictObjectJSONSchema
	}
	return string(out)
}

func normalizeStrictSchemaNode(node any) any {
	switch value := node.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(value))
		for key, raw := range value {
			if raw == nil {
				switch key {
				case "items":
					normalized[key] = map[string]any{"type": "string"}
				case "properties":
					normalized[key] = map[string]any{}
				}
				continue
			}

			if key == "required" {
				if req := normalizeStringArray(raw); len(req) > 0 {
					normalized[key] = req
				}
				continue
			}

			next := normalizeStrictSchemaNode(raw)
			if next == nil {
				switch key {
				case "items":
					normalized[key] = map[string]any{"type": "string"}
				case "properties":
					normalized[key] = map[string]any{}
				}
				continue
			}
			normalized[key] = next
		}

		schemaType, _ := normalized["type"].(string)
		schemaType = strings.TrimSpace(schemaType)
		if schemaType == "" {
			if _, hasProperties := normalized["properties"]; hasProperties {
				normalized["type"] = "object"
				schemaType = "object"
			}
		}
		if schemaType == "array" {
			if _, ok := normalized["items"]; !ok {
				normalized["items"] = map[string]any{"type": "string"}
			}
		}
		if schemaType == "object" {
			if _, ok := normalized["properties"].(map[string]any); !ok {
				normalized["properties"] = map[string]any{}
			}
			normalized["additionalProperties"] = false
		}
		return normalized
	case []any:
		out := make([]any, 0, len(value))
		for _, item := range value {
			next := normalizeStrictSchemaNode(item)
			if next != nil {
				out = append(out, next)
			}
		}
		return out
	default:
		return value
	}
}

func requireAllPropertiesForObjects(node any) {
	switch value := node.(type) {
	case map[string]any:
		for _, raw := range value {
			requireAllPropertiesForObjects(raw)
		}

		schemaType, _ := value["type"].(string)
		if schemaType != "object" {
			return
		}
		properties, ok := value["properties"].(map[string]any)
		if !ok {
			value["required"] = []string{}
			return
		}
		keys := make([]string, 0, len(properties))
		for key := range properties {
			key = strings.TrimSpace(key)
			if key != "" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		value["required"] = keys
	case []any:
		for _, item := range value {
			requireAllPropertiesForObjects(item)
		}
	}
}

func normalizeStringArray(value any) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if str, ok := item.(string); ok {
				str = strings.TrimSpace(str)
				if str != "" {
					out = append(out, str)
				}
			}
		}
		return out
	default:
		return nil
	}
}
