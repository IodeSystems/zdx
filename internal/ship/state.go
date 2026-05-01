package ship

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// stateFilePath returns the path where completed-stage state is persisted
// for a given git SHA, component name, and pass name (e.g. "current"/"next").
// stateDir overrides the default .zdx/ship-state base directory; pass ""
// to use the default.
func stateFilePath(stateDir, sha, comp, pass string) string {
	if stateDir == "" {
		stateDir = filepath.Join(".zdx", "ship-state")
	}
	return filepath.Join(stateDir, sha+"-"+comp+"-"+pass+".json")
}

// loadCompletedStages reads the JSON stage-name array at path and returns
// it as a set. Returns an empty set (not an error) when the file is absent.
func loadCompletedStages(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m, nil
}

// appendCompletedStage appends stageName to the JSON array at path,
// creating the file (and parent directory) if needed. Idempotent: if
// stageName is already recorded, the file is unchanged. Writes are
// atomic (write to .tmp, rename).
func appendCompletedStage(path, stageName string) error {
	data, _ := os.ReadFile(path)
	var names []string
	_ = json.Unmarshal(data, &names)
	for _, n := range names {
		if n == stageName {
			return nil
		}
	}
	names = append(names, stageName)
	out, err := json.Marshal(names)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
