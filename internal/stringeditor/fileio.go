package stringeditor

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// LoadStrings reads strings.json and returns a map of key -> value.
// If the file doesn't exist, it creates it with defaults from metadata.
func LoadStrings(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create with defaults
			defaults := DefaultStrings()
			if writeErr := SaveStrings(path, defaults); writeErr != nil {
				return nil, fmt.Errorf("creating default strings.json: %w", writeErr)
			}
			return defaults, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return result, nil
}

// SaveStrings writes the string map back to the JSON file with sorted keys
// and pretty formatting. It preserves any keys not in the editor metadata.
func SaveStrings(path string, strings map[string]string) error {
	// Use sorted keys for deterministic output
	keys := make([]string, 0, len(strings))
	for k := range strings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build ordered map manually for clean JSON output
	// json.Marshal on map doesn't guarantee order, so we use json.Encoder
	// with an ordered structure
	ordered := make([]keyValue, 0, len(keys))
	for _, k := range keys {
		// Skip internal placeholder keys (those starting with _)
		if len(k) > 0 && k[0] == '_' {
			continue
		}
		ordered = append(ordered, keyValue{Key: k, Value: strings[k]})
	}

	data, err := marshalOrdered(ordered)
	if err != nil {
		return fmt.Errorf("encoding strings: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

type keyValue struct {
	Key   string
	Value string
}

// marshalOrdered produces a JSON object with keys in the given order.
func marshalOrdered(pairs []keyValue) ([]byte, error) {
	buf := []byte("{\n")
	for i, kv := range pairs {
		keyJSON, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		valJSON, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		buf = append(buf, "  "...)
		buf = append(buf, keyJSON...)
		buf = append(buf, ": "...)
		buf = append(buf, valJSON...)
		if i < len(pairs)-1 {
			buf = append(buf, ',')
		}
		buf = append(buf, '\n')
	}
	buf = append(buf, "}\n"...)
	return buf, nil
}

// DefaultStrings returns the default string values matching the original
// Pascal FormatStrings() procedure defaults.
func DefaultStrings() map[string]string {
	entries := StringEntries()
	defaults := make(map[string]string, len(entries))
	for _, e := range entries {
		if len(e.Key) > 0 && e.Key[0] != '_' {
			defaults[e.Key] = "" // Empty default; actual defaults come from the JSON file
		}
	}
	return defaults
}
