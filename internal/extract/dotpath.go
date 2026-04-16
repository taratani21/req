package extract

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func DotPath(data []byte, path string) (string, error) {
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	parts := strings.Split(path, ".")
	current := root

	for _, part := range parts {
		switch node := current.(type) {
		case map[string]interface{}:
			val, ok := node[part]
			if !ok {
				return "", fmt.Errorf("key %q not found at path %q", part, path)
			}
			current = val
		case []interface{}:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return "", fmt.Errorf("expected numeric index for array, got %q at path %q", part, path)
			}
			if idx < 0 || idx >= len(node) {
				return "", fmt.Errorf("index %d out of range (length %d) at path %q", idx, len(node), path)
			}
			current = node[idx]
		default:
			return "", fmt.Errorf("cannot traverse into %T at part %q of path %q", current, part, path)
		}
	}

	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "null", nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cannot serialize value at path %q: %w", path, err)
		}
		return string(b), nil
	}
}

func All(data []byte, mapping map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(mapping))
	for varName, dotPath := range mapping {
		val, err := DotPath(data, dotPath)
		if err != nil {
			return nil, fmt.Errorf("extracting %q (path %q): %w", varName, dotPath, err)
		}
		result[varName] = val
	}
	return result, nil
}
