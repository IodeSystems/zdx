package doctor

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// Stack describes the detected toolchain of a project on disk.
type Stack struct {
	HasGo       bool
	GoVersion   string // major.minor, e.g. "1.25"
	HasNode     bool
	NodeVersion string // major, e.g. "20"
	HasPython   bool
	PythonMinor string // major.minor, e.g. "3.12"
	HasSqlc     bool
	HasPostgres bool
}

// DetectStack inspects root for toolchain signals.
func DetectStack(root string) Stack {
	s := Stack{}
	if v, ok := readGoVersion(root); ok {
		s.HasGo = true
		s.GoVersion = v
		s.HasPostgres = goModHasPostgres(root)
	}
	if v, ok := readNodeVersion(root); ok {
		s.HasNode = true
		s.NodeVersion = v
	}
	if v, ok := readPythonVersion(root); ok {
		s.HasPython = true
		s.PythonMinor = v
	}
	if fileExistsAt(root, "sqlc.yaml") || fileExistsAt(root, "sqlc.yml") || fileExistsAt(root, "sqlc.json") {
		s.HasSqlc = true
	}
	return s
}

var goDirectiveRe = regexp.MustCompile(`(?m)^go\s+(\d+\.\d+)`)

func readGoVersion(root string) (string, bool) {
	data, err := os.ReadFile(joinPath(root, "go.mod"))
	if err != nil {
		return "", false
	}
	if m := goDirectiveRe.FindSubmatch(data); m != nil {
		return string(m[1]), true
	}
	return "1.25", true
}

var postgresDriverRe = regexp.MustCompile(`(?m)\b(pgvector/pgvector-go|lib/pq|jackc/pgx|jackc/pgx/v\d+)\b`)

func goModHasPostgres(root string) bool {
	data, err := os.ReadFile(joinPath(root, "go.mod"))
	if err != nil {
		return false
	}
	return postgresDriverRe.Match(data)
}

func readNodeVersion(root string) (string, bool) {
	for _, p := range []string{"package.json", "ui/package.json"} {
		data, err := os.ReadFile(joinPath(root, p))
		if err != nil {
			continue
		}
		var pkg struct {
			Engines struct {
				Node string `json:"node"`
			} `json:"engines"`
		}
		if err := json.Unmarshal(data, &pkg); err == nil {
			if v := extractMajor(pkg.Engines.Node); v != "" {
				return v, true
			}
		}
		return "20", true
	}
	return "", false
}

var digitsRe = regexp.MustCompile(`\d+`)

func extractMajor(spec string) string {
	if spec == "" {
		return ""
	}
	if m := digitsRe.FindString(spec); m != "" {
		return m
	}
	return ""
}

var pythonVersionRe = regexp.MustCompile(`(?m)python[_-]?requires\s*[=:]\s*["']?[><=~^]*\s*(\d+\.\d+)`)

func readPythonVersion(root string) (string, bool) {
	if data, err := os.ReadFile(joinPath(root, "pyproject.toml")); err == nil {
		if m := pythonVersionRe.FindSubmatch(data); m != nil {
			return string(m[1]), true
		}
		return "3.12", true
	}
	if _, err := os.Stat(joinPath(root, "requirements.txt")); err == nil {
		return "3.12", true
	}
	return "", false
}

func fileExistsAt(root, name string) bool {
	_, err := os.Stat(joinPath(root, name))
	return err == nil
}

func joinPath(root, name string) string {
	if root == "" || root == "." {
		return name
	}
	if strings.HasSuffix(root, "/") {
		return root + name
	}
	return root + "/" + name
}
