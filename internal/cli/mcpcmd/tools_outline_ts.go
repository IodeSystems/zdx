package mcpcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTSOutline registers a `ts_outline` tool that returns a regex-based
// structural summary of TypeScript / TSX source. Not a full parser — but
// captures the constructs an agent usually wants (imports, exports, top-level
// declarations) without spawning tsserver or pulling tree-sitter as a cgo
// dependency. For exact AST queries, fall back to read_file.
func registerTSOutline(srv *mcp.Server, root string) {
	type tsOutlineIn struct {
		Path string `json:"path,omitempty" jsonschema:"repo-relative .ts/.tsx/.js/.jsx file or directory (defaults to repo root)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ts_outline",
		Description: "Regex-based outline of TypeScript / TSX (also JS / JSX): imports, exports, top-level function/class/interface/type/enum/const/let/var declarations with line numbers. Best-effort, not a real parser; agents needing precise AST info should read_file. Accepts a single file or a directory (recursive, skips node_modules/, .git/, dist/, build/).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in tsOutlineIn) (*mcp.CallToolResult, any, error) {
		target := in.Path
		if target == "" {
			target = "."
		}
		abs, err := resolveInRoot(root, target)
		if err != nil {
			return nil, nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, nil, err
		}
		var paths []string
		if info.IsDir() {
			_ = filepath.Walk(abs, func(p string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil || fi == nil {
					return nil
				}
				if fi.IsDir() {
					name := filepath.Base(p)
					if name == ".git" || name == "node_modules" || name == "dist" || name == "build" {
						return filepath.SkipDir
					}
					return nil
				}
				if isTSFile(p) {
					paths = append(paths, p)
				}
				return nil
			})
		} else {
			if !isTSFile(abs) {
				return nil, nil, fmt.Errorf("not a .ts/.tsx/.js/.jsx file: %s", target)
			}
			paths = []string{abs}
		}

		var files []map[string]any
		for _, p := range paths {
			rel, _ := filepath.Rel(root, p)
			data, readErr := os.ReadFile(p)
			if readErr != nil {
				files = append(files, map[string]any{"path": rel, "error": readErr.Error()})
				continue
			}
			files = append(files, summarizeTSFile(rel, string(data)))
		}
		return nil, map[string]any{"path": target, "files": files}, nil
	})
}

func isTSFile(p string) bool {
	switch filepath.Ext(p) {
	case ".ts", ".tsx", ".js", ".jsx", ".mts", ".cts":
		return true
	}
	return false
}

// Regexes are intentionally permissive — easier to capture too much than to
// drop a useful declaration.
var (
	tsImportSimpleRE    = regexp.MustCompile(`^\s*import\s+(?:type\s+)?(?:[^"';]+?)\s+from\s+["']([^"']+)["']`)
	tsImportSideRE      = regexp.MustCompile(`^\s*import\s+["']([^"']+)["']`)
	tsExportFromRE      = regexp.MustCompile(`^\s*export\s+(?:type\s+)?(?:\*|\{[^}]*\})\s+from\s+["']([^"']+)["']`)
	tsExportDefaultRE   = regexp.MustCompile(`^\s*export\s+default\s+(?:async\s+)?(?:(function|class)\s+(\w+)|(\w+))?`)
	tsExportNamedDeclRE = regexp.MustCompile(`^\s*export\s+(?:default\s+)?(?:abstract\s+)?(?:async\s+)?(function\*?|class|interface|type|enum|const|let|var)\s+(\w+)`)
	tsTopDeclRE         = regexp.MustCompile(`^(?:async\s+)?(function\*?|class|interface|type|enum)\s+(\w+)`)
)

func summarizeTSFile(rel, content string) map[string]any {
	var imports []map[string]any
	var decls []map[string]any

	// Strip block comments to avoid false matches inside doc blocks. We keep
	// line counts aligned by replacing comment bytes with spaces (preserving
	// newlines).
	stripped := stripBlockComments(content)
	// Multi-line imports: a line with `import` but no `from` should accumulate
	// until we see `from "..."` or `";"`. Cheap version — if a line starts
	// with `import` and contains no quote, peek forward.
	lines := strings.Split(stripped, "\n")
	for i := 0; i < len(lines); i++ {
		raw := stripLineComment(lines[i])
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		// Multi-line import accumulation.
		if strings.HasPrefix(trimmed, "import") && !strings.Contains(trimmed, "from") && !strings.Contains(trimmed, "\"") && !strings.Contains(trimmed, "'") {
			joined := raw
			for j := i + 1; j < len(lines) && j < i+20; j++ {
				joined += " " + strings.TrimSpace(stripLineComment(lines[j]))
				if strings.Contains(joined, "from") && (strings.Contains(joined, "\"") || strings.Contains(joined, "'")) {
					break
				}
			}
			raw = joined
			trimmed = strings.TrimSpace(joined)
		}

		if m := tsImportSimpleRE.FindStringSubmatch(raw); m != nil {
			imports = append(imports, map[string]any{"line": i + 1, "from": m[1]})
			continue
		}
		if m := tsImportSideRE.FindStringSubmatch(raw); m != nil {
			imports = append(imports, map[string]any{"line": i + 1, "from": m[1], "side_effect": true})
			continue
		}
		if m := tsExportFromRE.FindStringSubmatch(raw); m != nil {
			decls = append(decls, map[string]any{"kind": "re-export", "line": i + 1, "from": m[1]})
			continue
		}
		if m := tsExportDefaultRE.FindStringSubmatch(raw); m != nil {
			entry := map[string]any{"kind": "export-default", "line": i + 1, "exported": true}
			if m[1] != "" && m[2] != "" {
				entry["sub_kind"] = m[1]
				entry["name"] = m[2]
			} else if m[3] != "" {
				entry["name"] = m[3]
			}
			decls = append(decls, entry)
			continue
		}
		if m := tsExportNamedDeclRE.FindStringSubmatch(raw); m != nil {
			decls = append(decls, map[string]any{
				"kind":      m[1],
				"name":      m[2],
				"line":      i + 1,
				"exported":  true,
				"signature": condenseSignature(raw),
			})
			continue
		}
		// Only catch top-level (column-0 ish) unexported declarations to avoid
		// false matches on nested funcs/classes. Allow up to 2 leading spaces
		// for files using tabs-then-spaces oddities.
		leading := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if leading <= 2 {
			if m := tsTopDeclRE.FindStringSubmatch(strings.TrimLeft(raw, " \t")); m != nil {
				decls = append(decls, map[string]any{
					"kind":      m[1],
					"name":      m[2],
					"line":      i + 1,
					"exported":  false,
					"signature": condenseSignature(raw),
				})
			}
		}
	}

	return map[string]any{
		"path":    rel,
		"imports": imports,
		"decls":   decls,
	}
}

// stripBlockComments returns content with /* ... */ regions replaced by spaces
// (preserving newlines so line numbers stay aligned). Naive — does not honor
// strings or template literals containing the comment markers.
func stripBlockComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			b.WriteString("  ")
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				if s[i] == '\n' {
					b.WriteByte('\n')
				} else {
					b.WriteByte(' ')
				}
				i++
			}
			if i+1 < len(s) {
				b.WriteString("  ")
				i += 2
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func stripLineComment(line string) string {
	// Find // outside of strings. Cheap: only strip when we encounter // not
	// inside an obvious "..." or '...'. Misses template-literal cases.
	inSingle, inDouble := false, false
	for i := 0; i < len(line)-1; i++ {
		c := line[i]
		switch {
		case c == '\\' && i+1 < len(line):
			i++
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '/' && line[i+1] == '/' && !inSingle && !inDouble:
			return line[:i]
		}
	}
	return line
}

func condenseSignature(raw string) string {
	s := strings.TrimSpace(raw)
	// Drop trailing ` { ` opener so signatures end cleanly.
	if i := strings.IndexByte(s, '{'); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	// Cap length so sigs don't dominate the outline payload.
	const max = 200
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}
