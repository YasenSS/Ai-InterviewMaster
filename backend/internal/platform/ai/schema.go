package ai

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// ValidateJSON checks data against a subset of JSON Schema draft 2020-12 used
// by prompt packages: type, properties, required, additionalProperties,
// min/max items, min/max length, and numeric minimum/maximum.
func ValidateJSON(schema []byte, data []byte) error {
	if len(schema) == 0 {
		return nil
	}
	var spec map[string]any
	if err := json.Unmarshal(schema, &spec); err != nil {
		return fmt.Errorf("decode JSON Schema: %w", err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode JSON payload: %w", err)
	}
	return validateValue("$", spec, value)
}

func validateValue(path string, schema map[string]any, value any) error {
	expectedType, _ := schema["type"].(string)
	switch expectedType {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		return validateObject(path, schema, object)
	case "array":
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array", path)
		}
		return validateArray(path, schema, array)
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: expected string", path)
		}
		return validateString(path, schema, text)
	case "integer":
		number, ok := asInt(value)
		if !ok {
			return fmt.Errorf("%s: expected integer", path)
		}
		return validateNumber(path, schema, float64(number))
	case "number":
		number, ok := asFloat(value)
		if !ok {
			return fmt.Errorf("%s: expected number", path)
		}
		return validateNumber(path, schema, number)
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean", path)
		}
	}
	return nil
}

func validateObject(path string, schema map[string]any, object map[string]any) error {
	properties, _ := schema["properties"].(map[string]any)
	if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
		for key := range object {
			if _, exists := properties[key]; !exists {
				return fmt.Errorf("%s: unexpected property %q", path, key)
			}
		}
	}
	if required, ok := schema["required"].([]any); ok {
		for _, item := range required {
			key, _ := item.(string)
			if _, exists := object[key]; !exists {
				return fmt.Errorf("%s: missing property %q", path, key)
			}
		}
	}
	for key, raw := range properties {
		childSchema, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		child, exists := object[key]
		if !exists {
			continue
		}
		if err := validateValue(path+"."+key, childSchema, child); err != nil {
			return err
		}
	}
	return nil
}

func validateArray(path string, schema map[string]any, array []any) error {
	if min, ok := asInt(schema["minItems"]); ok && len(array) < min {
		return fmt.Errorf("%s: expected at least %d items", path, min)
	}
	if max, ok := asInt(schema["maxItems"]); ok && len(array) > max {
		return fmt.Errorf("%s: expected at most %d items", path, max)
	}
	itemSchema, ok := schema["items"].(map[string]any)
	if !ok {
		return nil
	}
	for index, item := range array {
		if err := validateValue(fmt.Sprintf("%s[%d]", path, index), itemSchema, item); err != nil {
			return err
		}
	}
	return nil
}

func validateString(path string, schema map[string]any, text string) error {
	count := utf8.RuneCountInString(text)
	if min, ok := asInt(schema["minLength"]); ok && count < min {
		return fmt.Errorf("%s: string shorter than %d", path, min)
	}
	if max, ok := asInt(schema["maxLength"]); ok && count > max {
		return fmt.Errorf("%s: string longer than %d", path, max)
	}
	return nil
}

func validateNumber(path string, schema map[string]any, number float64) error {
	if min, ok := asFloat(schema["minimum"]); ok && number < min {
		return fmt.Errorf("%s: value below minimum %v", path, min)
	}
	if max, ok := asFloat(schema["maximum"]); ok && number > max {
		return fmt.Errorf("%s: value above maximum %v", path, max)
	}
	return nil
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	}
	return 0, false
}

func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	}
	return 0, false
}
