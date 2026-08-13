package ai

import "testing"

func TestValidateJSONRejectsUnknownFieldsAndRangeErrors(t *testing.T) {
	schema := []byte(`{
		"type":"object",
		"additionalProperties":false,
		"properties":{
			"name":{"type":"string","minLength":1},
			"score":{"type":"integer","minimum":0,"maximum":100}
		},
		"required":["name","score"]
	}`)
	if err := ValidateJSON(schema, []byte(`{"name":"ok","score":80}`)); err != nil {
		t.Fatalf("valid payload rejected: %v", err)
	}
	if err := ValidateJSON(schema, []byte(`{"name":"ok","score":80,"extra":true}`)); err == nil {
		t.Fatal("unknown field accepted")
	}
	if err := ValidateJSON(schema, []byte(`{"name":"ok","score":120}`)); err == nil {
		t.Fatal("out of range score accepted")
	}
}
