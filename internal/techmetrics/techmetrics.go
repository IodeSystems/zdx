package techmetrics

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type TechMetrics struct {
	GoFiles         int `json:"go_files"`
	GoLOC           int `json:"go_loc"`
	TestFiles       int `json:"test_files"`
	TestFunctions   int `json:"test_functions"`
	Migrations      int `json:"migrations"`
	SQLQueryFiles   int `json:"sql_query_files"`
	TSFiles         int `json:"ts_files"`
	TSXFiles        int `json:"tsx_files"`
	TSLOC           int `json:"ts_loc"`
	GitCommits      int `json:"git_commits_since"`
	GitFilesChanged int `json:"git_files_changed_since"`
}

type MetricDelta struct {
	Name string `json:"name"`
	Prev int    `json:"prev"`
	Curr int    `json:"curr"`
	Diff int    `json:"diff"`
}

func Collect(projectRoot string) TechMetrics {
	m := TechMetrics{}

	m.GoFiles = countFilesByGlob(projectRoot, "**/*.go", "_test.go")
	m.GoLOC = countLOCByExt(projectRoot, ".go")
	m.TestFiles = countFilesByPattern(projectRoot, "**/*_test.go")
	m.TestFunctions = countTestFunctions(projectRoot)
	m.Migrations = countFilesByPattern(projectRoot, "internal/migrate/sql/*.up.sql")
	m.SQLQueryFiles = countFilesByPattern(projectRoot, "queries/*.sql")
	m.TSFiles = countFilesByGlob(projectRoot, "ui/src/**/*.ts", ".tsx")
	m.TSXFiles = countFilesByPattern(projectRoot, "ui/src/**/*.tsx")
	m.TSLOC = countLOCInDir(filepath.Join(projectRoot, "ui", "src"))

	return m
}

func CollectGitChurn(projectRoot, sinceDate string) (commits int, filesChanged int) {
	if sinceDate == "" {
		return 0, 0
	}
	out, err := exec.Command("git", "-C", projectRoot, "log", "--oneline", "--since="+sinceDate).Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if lines[0] != "" {
			commits = len(lines)
		}
	}
	out2, err2 := exec.Command("git", "-C", projectRoot, "log", "--name-only", "--pretty=format:", "--since="+sinceDate).Output()
	if err2 == nil {
		lines := strings.Split(strings.TrimSpace(string(out2)), "\n")
		seen := map[string]bool{}
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				seen[l] = true
			}
		}
		filesChanged = len(seen)
	}
	return
}

func ComputeDeltas(prev, curr TechMetrics) []MetricDelta {
	pairs := []struct {
		name string
		p, c int
	}{
		{"go_files", prev.GoFiles, curr.GoFiles},
		{"go_loc", prev.GoLOC, curr.GoLOC},
		{"test_files", prev.TestFiles, curr.TestFiles},
		{"test_functions", prev.TestFunctions, curr.TestFunctions},
		{"migrations", prev.Migrations, curr.Migrations},
		{"sql_query_files", prev.SQLQueryFiles, curr.SQLQueryFiles},
		{"ts_files", prev.TSFiles, curr.TSFiles},
		{"tsx_files", prev.TSXFiles, curr.TSXFiles},
		{"ts_loc", prev.TSLOC, curr.TSLOC},
	}
	var deltas []MetricDelta
	for _, p := range pairs {
		deltas = append(deltas, MetricDelta{
			Name: p.name,
			Prev: p.p,
			Curr: p.c,
			Diff: p.c - p.p,
		})
	}
	return deltas
}

func ToJSON(m TechMetrics) string {
	b, _ := json.Marshal(m)
	return string(b)
}

func DeltasToJSON(d []MetricDelta) string {
	b, _ := json.Marshal(d)
	return string(b)
}

func Parse(stateJSON string) (TechMetrics, bool) {
	if stateJSON == "" || stateJSON == "{}" {
		return TechMetrics{}, false
	}
	var m TechMetrics
	if err := json.Unmarshal([]byte(stateJSON), &m); err != nil {
		return TechMetrics{}, false
	}
	return m, true
}

func FormatSummary(m TechMetrics, deltas []MetricDelta) string {
	var sb strings.Builder
	for _, d := range deltas {
		sign := ""
		if d.Diff > 0 {
			sign = "+"
		}
		diffStr := ""
		if d.Prev != 0 || d.Curr != 0 {
			if d.Diff != 0 {
				diffStr = fmt.Sprintf(" (%s%d)", sign, d.Diff)
			}
		}
		sb.WriteString(fmt.Sprintf("  %-20s %d%s\n", metricLabel(d.Name), d.Curr, diffStr))
	}
	return sb.String()
}

func metricLabel(name string) string {
	labels := map[string]string{
		"go_files":        "Go files",
		"go_loc":          "Go LOC",
		"test_files":      "Test files",
		"test_functions":  "Test functions",
		"migrations":      "Migrations",
		"sql_query_files": "SQL queries",
		"ts_files":        "TS files",
		"tsx_files":       "TSX files",
		"ts_loc":          "TS/TSX LOC",
	}
	if l, ok := labels[name]; ok {
		return l
	}
	return name
}

func countFilesByGlob(root, pattern, exclude string) int {
	matches, _ := filepath.Glob(filepath.Join(root, pattern))
	if matches == nil {
		out, err := exec.Command("find", root, "-name", extractGlobName(pattern), "-type", "f").Output()
		if err != nil {
			return 0
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		count := 0
		for _, l := range lines {
			if l != "" && (exclude == "" || !strings.HasSuffix(l, exclude)) {
				count++
			}
		}
		return count
	}
	count := 0
	for _, m := range matches {
		if exclude == "" || !strings.HasSuffix(m, exclude) {
			count++
		}
	}
	return count
}

func countFilesByPattern(root, pattern string) int {
	full := filepath.Join(root, pattern)
	matches, _ := filepath.Glob(full)
	if matches != nil {
		return len(matches)
	}
	parts := strings.SplitN(pattern, "/", -1)
	name := parts[len(parts)-1]
	dir := root
	if len(parts) > 1 {
		dir = filepath.Join(root, filepath.Join(parts[:len(parts)-1]...))
	}
	out, err := exec.Command("find", dir, "-name", name, "-type", "f").Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if lines[0] == "" {
		return 0
	}
	return len(lines)
}

func countTestFunctions(root string) int {
	out, err := exec.Command("grep", "-r", "--include=*_test.go", "-c", "func Test", root).Output()
	if err != nil {
		return 0
	}
	total := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			n, _ := strconv.Atoi(parts[len(parts)-1])
			total += n
		}
	}
	return total
}

func countLOCByExt(root, ext string) int {
	out, err := exec.Command("find", root, "-name", "*"+ext, "-type", "f",
		"-not", "-path", "*/vendor/*", "-not", "-path", "*/.git/*",
		"-not", "-name", "*_generated*").Output()
	if err != nil {
		return 0
	}
	total := 0
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		total += len(strings.Split(string(data), "\n"))
	}
	return total
}

func countLOCInDir(dir string) int {
	if _, err := os.Stat(dir); err != nil {
		return 0
	}
	out, err := exec.Command("find", dir, "-type", "f", "(",
		"-name", "*.ts", "-o", "-name", "*.tsx", ")").Output()
	if err != nil {
		return 0
	}
	total := 0
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		total += len(strings.Split(string(data), "\n"))
	}
	return total
}

func extractGlobName(pattern string) string {
	parts := strings.Split(pattern, "/")
	return parts[len(parts)-1]
}
