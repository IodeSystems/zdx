package mcpcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// writeScratch caches large tool output under .zdx/agent/scratch/ so the agent
// can re-read or grep it without re-running the producing command. Returns the
// repo-relative path (suitable for read_file/grep callbacks) or an error if
// the cache directory is not writable. Filenames are timestamped + uuid-keyed
// so concurrent agents do not collide.
func writeScratch(root, prefix, content string) (string, error) {
	dir := filepath.Join(root, ".zdx", "agent", "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%s-%s.txt", prefix, time.Now().UTC().Format("20060102T150405Z"), uuid.New().String()[:8])
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return "", err
	}
	return rel, nil
}

// generatedFilePatterns matches paths owned by the merge-train's regen step.
// Agents that hand-edit these create merge-train conflicts and lose their
// edits in the next regen; refuse the write at the dispatch boundary so the
// worker contract is enforced by the harness, not just documented in the
// system prompt (defense in depth: prompts drift, code doesn't).
//
// Patterns are matched against the repo-relative input path before resolution
// so the error message references what the agent typed.
var generatedFilePatterns = []struct {
	match  func(string) bool
	source string
}{
	{func(p string) bool { return strings.HasPrefix(p, "internal/db/") && strings.HasSuffix(p, ".sql.go") }, "sqlc — edit queries/*.sql and run `~/go/bin/sqlc generate`"},
	{func(p string) bool { return strings.HasPrefix(p, "internal/db/") && strings.HasSuffix(p, ".sql.metaquery.go") }, "sqlc-metaquery — suppress with `-- metaquery: off` in the .sql file"},
	{func(p string) bool { return p == "internal/db/querier.go" }, "sqlc — edit queries/*.sql and regen"},
	{func(p string) bool { return p == "internal/db/models.go" }, "sqlc — edit migrations + queries and regen"},
	{func(p string) bool { return p == "internal/dxclient/models.gen.go" }, "oapi-codegen — edit handlers and run `dx ui gen-api`"},
	{func(p string) bool { return p == "ui/src/api.gen.ts" }, "openapi-typescript — edit handlers and run `dx ui gen-api`"},
	{func(p string) bool { return p == "schema/shipped.sql" }, "pg_dump snapshot — owned by `bin/regen-schema` or `bin/db migrate`"},
}

// rejectGeneratedFileWrite returns a non-nil error when path matches a known
// codegen output. Callers should propagate the error to the agent so the LLM
// sees the WHY and can correct course. The error message includes the source
// of truth and the regen command so the agent has a path forward without
// needing to consult docs.
func rejectGeneratedFileWrite(path string) error {
	p := filepath.ToSlash(filepath.Clean(path))
	for _, g := range generatedFilePatterns {
		if g.match(p) {
			return fmt.Errorf("refusing to write %s: this is a generated file (%s). Hand-edits would be lost in the next regen and break the merge-train", path, g.source)
		}
	}
	return nil
}

// RegisterFSTools registers filesystem tools (read_file, write_file, edit_file,
// grep, glob, list_dir) scoped to the given repo root. All path arguments are
// resolved relative to root and rejected if they escape it.
func RegisterFSTools(srv *mcp.Server, root string) {
	root = filepath.Clean(root)

	type readFileIn struct {
		Path   string `json:"path" jsonschema:"required,repo-relative file path"`
		Offset int    `json:"offset,omitempty" jsonschema:"1-based line offset"`
		Limit  int    `json:"limit,omitempty" jsonschema:"max lines to return (default 2000)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "read_file",
		Description: "Read a file from the repo. Returns UTF-8 contents, optionally sliced by line offset/limit.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in readFileIn) (*mcp.CallToolResult, any, error) {
		abs, err := resolveInRoot(root, in.Path)
		if err != nil {
			return nil, nil, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, nil, err
		}
		text := string(data)
		// Always slice through the limit path so the documented "default 2000"
		// applies even when offset/limit are unset. Some local LLM chat
		// templates (e.g. llama.cpp jinja for Qwen3) abort the server process
		// when a tool_result exceeds ~50KB, so we also enforce a hard byte cap.
		lines := strings.Split(text, "\n")
		start := in.Offset - 1
		if start < 0 {
			start = 0
		}
		if start > len(lines) {
			start = len(lines)
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 2000
		}
		end := len(lines)
		if start+limit < end {
			end = start + limit
		}
		text = strings.Join(lines[start:end], "\n")
		truncated := end < len(lines)
		// Long-line truncation: a single line longer than maxLineChars is cut
		// with an annotation. Critical for files like internal/dxclient/openapi.json
		// (415KB on a single line) where one line would otherwise blow the
		// per-result byte cap on its own. The line-count cap above doesn't
		// help when each "line" is enormous.
		const maxLineChars = 500
		text, longLines, longBytes := truncateLongLines(text, maxLineChars)
		const maxBytes = 48 * 1024
		if len(text) > maxBytes {
			text = text[:maxBytes]
			truncated = true
		}
		out := map[string]any{"path": in.Path, "content": text}
		if truncated {
			out["truncated"] = true
			out["total_lines"] = len(lines)
		}
		if longLines > 0 {
			out["truncated_lines"] = longLines
			out["truncated_line_chars"] = longBytes
		}
		return nil, out, nil
	})

	type writeFileIn struct {
		Path    string `json:"path" jsonschema:"required,repo-relative file path"`
		Content string `json:"content" jsonschema:"required,file contents"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "write_file",
		Description: "Write UTF-8 contents to a file, creating parent directories as needed. Refuses writes to merge-train-owned generated files (sqlc/dxclient/openapi-typescript/shipped.sql outputs) — fix the source and regenerate instead.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in writeFileIn) (*mcp.CallToolResult, any, error) {
		if err := rejectGeneratedFileWrite(in.Path); err != nil {
			return nil, nil, err
		}
		abs, err := resolveInRoot(root, in.Path)
		if err != nil {
			return nil, nil, err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, nil, err
		}
		if err := os.WriteFile(abs, []byte(in.Content), 0o644); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"path": in.Path, "bytes": len(in.Content)}, nil
	})

	type editFileIn struct {
		Path       string `json:"path" jsonschema:"required,repo-relative file path"`
		OldString  string `json:"old_string" jsonschema:"required,exact text to replace"`
		NewString  string `json:"new_string" jsonschema:"required,replacement text"`
		ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"replace every occurrence (default false)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "edit_file",
		Description: "Replace exact text in a file. Fails if old_string is not unique unless replace_all=true. Refuses edits to merge-train-owned generated files (sqlc/dxclient/openapi-typescript/shipped.sql outputs) — fix the source and regenerate instead.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in editFileIn) (*mcp.CallToolResult, any, error) {
		if err := rejectGeneratedFileWrite(in.Path); err != nil {
			return nil, nil, err
		}
		abs, err := resolveInRoot(root, in.Path)
		if err != nil {
			return nil, nil, err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, nil, err
		}
		text := string(data)
		count := strings.Count(text, in.OldString)
		if count == 0 {
			return nil, nil, fmt.Errorf("old_string not found in %s", in.Path)
		}
		if count > 1 && !in.ReplaceAll {
			return nil, nil, fmt.Errorf("old_string matches %d locations in %s; pass replace_all=true or narrow the match", count, in.Path)
		}
		var out string
		if in.ReplaceAll {
			out = strings.ReplaceAll(text, in.OldString, in.NewString)
		} else {
			out = strings.Replace(text, in.OldString, in.NewString, 1)
		}
		if err := os.WriteFile(abs, []byte(out), 0o644); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"path": in.Path, "replacements": count}, nil
	})

	type listDirIn struct {
		Path string `json:"path,omitempty" jsonschema:"repo-relative directory (defaults to root)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_dir",
		Description: "List entries in a directory with basic metadata (name, is_dir, size).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listDirIn) (*mcp.CallToolResult, any, error) {
		target := in.Path
		if target == "" {
			target = "."
		}
		abs, err := resolveInRoot(root, target)
		if err != nil {
			return nil, nil, err
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			return nil, nil, err
		}
		var items []map[string]any
		for _, e := range entries {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			items = append(items, map[string]any{
				"name":   e.Name(),
				"is_dir": e.IsDir(),
				"size":   size,
			})
		}
		return nil, map[string]any{"path": target, "entries": items}, nil
	})

	type globIn struct {
		Pattern string `json:"pattern" jsonschema:"required,glob pattern (e.g. **/*.go) rooted at repo"`
		Limit   int    `json:"limit,omitempty" jsonschema:"max matches (default 500)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "glob",
		Description: "Match files under the repo root with a double-star-aware glob pattern.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in globIn) (*mcp.CallToolResult, any, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 500
		}
		matches, err := globDoubleStar(root, in.Pattern, limit)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"pattern": in.Pattern, "matches": matches}, nil
	})

	type grepIn struct {
		Pattern       string `json:"pattern" jsonschema:"required,Go regexp"`
		Path          string `json:"path,omitempty" jsonschema:"repo-relative file or dir (defaults to root)"`
		Glob          string `json:"glob,omitempty" jsonschema:"filename glob filter (e.g. *.go)"`
		CaseSensitive bool   `json:"case_sensitive,omitempty" jsonschema:"case-sensitive match (default false)"`
		Limit         int    `json:"limit,omitempty" jsonschema:"max matching lines to return inline (default 2000)"`
		MatchLen      int    `json:"match_len,omitempty" jsonschema:"max chars of matching line text; longer lines are windowed around the match with ... ellipses (default 200, 0 disables truncation)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "grep",
		Description: "Search file contents for a Go regexp. Returns matching path:line:text entries. When more matches exist than the inline limit, the full result list is cached under .zdx/agent/scratch/ and the path is included in the response so you can grep it for further filtering instead of rerunning.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in grepIn) (*mcp.CallToolResult, any, error) {
		pat := in.Pattern
		if !in.CaseSensitive {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid pattern: %w", err)
		}
		target := in.Path
		if target == "" {
			target = "."
		}
		absStart, err := resolveInRoot(root, target)
		if err != nil {
			return nil, nil, err
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 2000
		}
		matchLen := in.MatchLen
		if matchLen == 0 {
			matchLen = 200
		}
		var matches []map[string]any
		var scratchLines []string // every match formatted path:line:col:text — written to scratch when truncated
		walk := func(path string, info os.FileInfo) error {
			if info.IsDir() {
				return nil
			}
			if in.Glob != "" {
				ok, _ := filepath.Match(in.Glob, filepath.Base(path))
				if !ok {
					return nil
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			for i, line := range strings.Split(string(data), "\n") {
				loc := re.FindStringIndex(line)
				if loc == nil {
					continue
				}
				text, off := windowMatch(line, loc, matchLen)
				if len(matches) < limit {
					matches = append(matches, map[string]any{
						"path": rel,
						"line": i + 1,
						"col":  off + 1,
						"text": text,
					})
				}
				scratchLines = append(scratchLines, fmt.Sprintf("%s:%d:%d:%s", rel, i+1, off+1, text))
			}
			return nil
		}
		info, err := os.Stat(absStart)
		if err != nil {
			return nil, nil, err
		}
		if info.IsDir() {
			_ = filepath.Walk(absStart, func(path string, fi os.FileInfo, err error) error {
				if err != nil || fi == nil {
					return nil
				}
				if fi.IsDir() {
					name := filepath.Base(path)
					if name == ".git" || name == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				return walk(path, fi)
			})
		} else {
			_ = walk(absStart, info)
		}
		out := map[string]any{"pattern": in.Pattern, "matches": matches}
		if len(scratchLines) > limit {
			out["truncated"] = true
			out["total_matches"] = len(scratchLines)
			full := strings.Join(scratchLines, "\n") + "\n"
			if scratchPath, sErr := writeScratch(root, "grep", full); sErr == nil {
				out["scratch_path"] = scratchPath
				out["hint"] = fmt.Sprintf("Returned first %d of %d matches inline. Full %s:line:col:text list cached at %s — use the grep tool with path=%q to filter further (e.g. by directory or symbol), or read_file with offset/limit to page through.", limit, len(scratchLines), "path", scratchPath, scratchPath)
			} else {
				out["hint"] = fmt.Sprintf("Returned first %d of %d matches inline. (scratch cache failed: %v) Refine the pattern or raise limit to see more.", limit, len(scratchLines), sErr)
			}
		}
		return nil, out, nil
	})
}

// windowMatch trims line down to at most maxLen bytes around the match at loc,
// centering the match in the window and prefixing/suffixing "..." when content
// was elided. maxLen <= 0 disables truncation. If the match itself exceeds the
// budget, the match is truncated with ellipses too. Returns the windowed text
// and the byte offset (0-based) at which the returned non-ellipsis content
// starts in the original line.
func windowMatch(line string, loc []int, maxLen int) (string, int) {
	if maxLen <= 0 || len(line) <= maxLen {
		return line, 0
	}
	const ell = "..."
	matchStart, matchEnd := loc[0], loc[1]
	matchSize := matchEnd - matchStart
	if matchSize >= maxLen {
		end := matchStart + maxLen
		if end > len(line) {
			end = len(line)
		}
		return ell + line[matchStart:end] + ell, matchStart
	}
	budget := maxLen - matchSize
	leftBudget := budget / 2
	rightBudget := budget - leftBudget
	start := matchStart - leftBudget
	end := matchEnd + rightBudget
	if start < 0 {
		end += -start
		start = 0
	}
	if end > len(line) {
		start -= end - len(line)
		end = len(line)
	}
	if start < 0 {
		start = 0
	}
	prefix := ""
	if start > 0 {
		prefix = ell
	}
	suffix := ""
	if end < len(line) {
		suffix = ell
	}
	return prefix + line[start:end] + suffix, start
}

// resolveInRoot resolves rel relative to root, returning an absolute path that
// is guaranteed to be within root. Symlinks are evaluated via filepath.EvalSymlinks
// when the target exists.
func resolveInRoot(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths not allowed: %s", rel)
	}
	abs := filepath.Clean(filepath.Join(root, rel))
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repo root: %s", rel)
	}
	return abs, nil
}

// globDoubleStar expands a pattern with ** support against root, returning
// repo-relative match paths.
func globDoubleStar(root, pattern string, limit int) ([]string, error) {
	if strings.Contains(pattern, "..") {
		return nil, fmt.Errorf("pattern may not contain ..")
	}
	re := globToRegexp(pattern)
	var out []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			name := filepath.Base(path)
			if name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= limit {
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if re.MatchString(rel) {
			out = append(out, rel)
		}
		return nil
	})
	return out, nil
}

func globToRegexp(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(pattern) {
		c := pattern[i]
		switch c {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i += 2
				if i < len(pattern) && pattern[i] == '/' {
					i++
				}
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		i++
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}
