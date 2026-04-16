package util

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestCleanJSONSchemaForStrictUpstream_StripsOneOfAndNormalizesArrayItems(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"rewardTitleEffects": {
				"type": "array",
				"items": {
					"oneOf": [
						{"type": "string"},
						{"type": "object", "properties": {"title": {"type": "string"}}}
					]
				}
			}
		},
		"required": ["rewardTitleEffects"]
	}`

	result := CleanJSONSchemaForStrictUpstream(input)

	if gjson.Get(result, "properties.rewardTitleEffects.items.oneOf").Exists() {
		t.Fatalf("oneOf should be removed: %s", result)
	}
	if got := gjson.Get(result, "properties.rewardTitleEffects.items.type").String(); got == "" {
		t.Fatalf("items.type should be normalized: %s", result)
	}
}

func TestCleanJSONSchemaForStrictUpstream_NormalizesNullArrayBits(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"sessions": {
				"type": "array",
				"items": null
			},
			"labels": {
				"type": ["array", "null"],
				"items": {"type": "string"}
			}
		},
		"required": null
	}`

	result := CleanJSONSchemaForStrictUpstream(input)

	if got := gjson.Get(result, "properties.sessions.items.type").String(); got == "" {
		t.Fatalf("sessions.items.type should be filled: %s", result)
	}
	if got := gjson.Get(result, "properties.labels.type").String(); got != "array" {
		t.Fatalf("expected labels.type=array, got %q: %s", got, result)
	}
	if gjson.Get(result, "required").Exists() {
		t.Fatalf("required should be removed when null: %s", result)
	}
}

func TestCleanJSONSchemaForStrictUpstream_EmptyFallsBackToObject(t *testing.T) {
	result := CleanJSONSchemaForStrictUpstream("")
	if got := gjson.Get(result, "type").String(); got != "object" {
		t.Fatalf("expected object fallback, got %q: %s", got, result)
	}
	if !gjson.Get(result, "properties").IsObject() {
		t.Fatalf("expected object fallback properties: %s", result)
	}
	if got := gjson.Get(result, "additionalProperties").Raw; got != "false" {
		t.Fatalf("expected additionalProperties=false on fallback, got %s", result)
	}
}

func TestCleanJSONSchemaForStrictUpstream_ForcesAdditionalPropertiesFalseRecursively(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"batch_product_result": {
				"type": "object",
				"properties": {
					"items": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"id": {"type": "string"}
							}
						}
					}
				}
			}
		}
	}`

	result := CleanJSONSchemaForStrictUpstream(input)

	if got := gjson.Get(result, "additionalProperties").Raw; got != "false" {
		t.Fatalf("expected root additionalProperties=false, got %s", result)
	}
	if got := gjson.Get(result, "properties.batch_product_result.additionalProperties").Raw; got != "false" {
		t.Fatalf("expected nested object additionalProperties=false, got %s", result)
	}
	if got := gjson.Get(result, "properties.batch_product_result.properties.items.items.additionalProperties").Raw; got != "false" {
		t.Fatalf("expected array item object additionalProperties=false, got %s", result)
	}
}

func TestCleanJSONSchemaForStrictUpstream_NormalizesSessionsListLikeSchema(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"sessions": {
				"type": "array",
				"items": null
			},
			"page": {
				"type": ["integer", "null"],
				"default": null
			}
		},
		"required": null
	}`

	result := CleanJSONSchemaForStrictUpstream(input)

	if got := gjson.Get(result, "properties.sessions.items.type").String(); got != "string" {
		t.Fatalf("expected sessions.items.type=string, got %q: %s", got, result)
	}
	if got := gjson.Get(result, "properties.page.type").String(); got != "integer" {
		t.Fatalf("expected page.type=integer, got %q: %s", got, result)
	}
	if gjson.Get(result, "properties.page.default").Exists() {
		t.Fatalf("default should be stripped from strict schema: %s", result)
	}
	if gjson.Get(result, "required").Exists() {
		t.Fatalf("required should be removed when null: %s", result)
	}
	if got := gjson.Get(result, "additionalProperties").Raw; got != "false" {
		t.Fatalf("expected additionalProperties=false, got %s", result)
	}
}
