package output

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type jsonPayload struct {
	Meta      Metadata         `json:"meta"`
	Resources []ResourceReport `json:"resources"`
}

// WriteJSON writes the discovered resources and metadata to a JSON file.
func WriteJSON(path string, reports []ResourceReport, meta Metadata) error {
	payload := jsonPayload{Meta: meta, Resources: reports}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	return os.WriteFile(path, data, 0o644)
}
