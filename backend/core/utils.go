package core

import "encoding/json"

// ParseConfig parses a map[string]interface{} into a target struct using JSON serialization.
// This is a standard utility to ensure safe configuration parsing across plugins.
func ParseConfig[T any](input map[string]interface{}) (*T, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
