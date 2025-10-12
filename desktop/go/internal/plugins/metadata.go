package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileMetadata represents the plugin metadata stored on disk.
type FileMetadata struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Persistent  map[string]interface{} `json:"persistent"`
}

// LoadMetadata reads and validates a metadata.json file.
func LoadMetadata(path string) (*FileMetadata, error) {
	if path == "" {
		return nil, fmt.Errorf("metadata path cannot be empty")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open metadata: %w", err)
	}
	defer file.Close()

	var metadata FileMetadata
	if err := json.NewDecoder(file).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}

	if metadata.Name == "" {
		return nil, fmt.Errorf("metadata missing name in %s", path)
	}
	if metadata.Version == "" {
		return nil, fmt.Errorf("metadata missing version in %s", path)
	}
	if metadata.Description == "" {
		return nil, fmt.Errorf("metadata missing description in %s", path)
	}

	if metadata.Persistent == nil {
		metadata.Persistent = make(map[string]interface{})
	}

	return &metadata, nil
}

// SaveMetadata persists metadata to disk with indentation for readability.
func SaveMetadata(path string, metadata *FileMetadata) error {
	if metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure metadata directory: %w", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace metadata: %w", err)
	}

	return nil
}

func clonePersistentState(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}

	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
