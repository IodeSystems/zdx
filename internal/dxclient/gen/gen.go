// Package gen produces the oapi-codegen input for internal/dxclient.
//
// Huma emits OpenAPI 3.1, which oapi-codegen v2 does not fully support. These
// helpers downgrade the spec to 3.0 (primarily rewriting `type: [X, null]` as
// `type: X, nullable: true`) and shell out to oapi-codegen against the
// downgraded spec.
//
// Callers need oapi-codegen on PATH:
//
//	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
package gen

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Generate downgrades the 3.1 spec at specPath to 3.0 and invokes oapi-codegen
// using configPath. Returns the output file path parsed from the config.
func Generate(specPath, configPath string) (string, error) {
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		return "", fmt.Errorf("read spec: %w", err)
	}

	downgraded, err := Downgrade31To30(specBytes)
	if err != nil {
		return "", fmt.Errorf("downgrade spec: %w", err)
	}

	tmp, err := os.CreateTemp("", "zdx-openapi-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(downgraded); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write temp: %w", err)
	}
	tmp.Close()

	bin := codegenBin()
	cmd := exec.Command(bin, "-config", configPath, tmp.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("oapi-codegen: %w (ensure it is on PATH: go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest)", err)
	}

	return readOutputPath(configPath), nil
}

func codegenBin() string {
	if p, err := exec.LookPath("oapi-codegen"); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "go", "bin", "oapi-codegen")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "oapi-codegen"
}

func readOutputPath(configPath string) string {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "output:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// Downgrade31To30 rewrites an OpenAPI 3.1 document into a 3.0-compatible form
// suitable for oapi-codegen v2. It handles the only 3.1 construct Huma emits
// today: union-style nullability (`type: [X, null]` → `type: X, nullable: true`).
// If Huma starts emitting other 3.1-specific keywords (unevaluatedProperties,
// dependentSchemas, const, etc.), extend this function.
func Downgrade31To30(src []byte) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	doc["openapi"] = "3.0.3"
	walk(doc)
	return json.Marshal(doc)
}

func walk(v any) {
	switch x := v.(type) {
	case map[string]any:
		if t, ok := x["type"]; ok {
			if arr, ok := t.([]any); ok {
				primary := ""
				nullable := false
				for _, e := range arr {
					s, ok := e.(string)
					if !ok {
						continue
					}
					if s == "null" {
						nullable = true
						continue
					}
					if primary == "" {
						primary = s
					}
				}
				if primary == "" {
					primary = "object"
				}
				x["type"] = primary
				if nullable {
					x["nullable"] = true
				}
			}
		}
		for _, vv := range x {
			walk(vv)
		}
	case []any:
		for _, vv := range x {
			walk(vv)
		}
	}
}
