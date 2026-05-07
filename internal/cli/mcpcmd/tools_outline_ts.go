package mcpcmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// outlineFileTS returns a regex-based outline of TS / TSX / JS / JSX source.
// Not a real parser — captures imports, exports, and top-level declarations.
// LOD semantics:
//
//	0 — {path, language}
//	1 — +decl_names
//	2 — +imports, decls with kind/name/line/exported (default)
//	3 — +signature, +import details (e.g. side_effect, sub_kind)
func outlineFileTS(rel, content string, lod int) map[string]any {
	out := map[string]any{
		"path":     rel,
		"language": tsLangFor(rel),
	}
	if lod < 1 {
		return out
	}

	stripped := stripBlockComments(content)
	lines := strings.Split(stripped, "\n")

	var imports []map[string]any
	var decls []map[string]any
	var declNames []string

	for i := 0; i < len(lines); i++ {
		raw := stripLineComment(lines[i])
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

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
			if lod >= 2 {
				imports = append(imports, map[string]any{"line": i + 1, "from": m[1]})
			}
			continue
		}
		if m := tsImportSideRE.FindStringSubmatch(raw); m != nil {
			if lod >= 2 {
				entry := map[string]any{"line": i + 1, "from": m[1]}
				if lod >= 3 {
					entry["side_effect"] = true
				}
				imports = append(imports, entry)
			}
			continue
		}
		if m := tsExportFromRE.FindStringSubmatch(raw); m != nil {
			declNames = append(declNames, "(re-export)")
			if lod >= 2 {
				decls = append(decls, map[string]any{"kind": "re-export", "line": i + 1, "from": m[1]})
			}
			continue
		}
		if m := tsExportDefaultRE.FindStringSubmatch(raw); m != nil {
			name := ""
			if m[1] != "" && m[2] != "" {
				name = m[2]
			} else if m[3] != "" {
				name = m[3]
			}
			if name != "" {
				declNames = append(declNames, name)
			}
			if lod >= 2 {
				entry := map[string]any{"kind": "export-default", "line": i + 1, "exported": true}
				if lod >= 3 && m[1] != "" && m[2] != "" {
					entry["sub_kind"] = m[1]
				}
				if name != "" {
					entry["name"] = name
				}
				decls = append(decls, entry)
			}
			continue
		}
		if m := tsExportNamedDeclRE.FindStringSubmatch(raw); m != nil {
			declNames = append(declNames, m[2])
			if lod >= 2 {
				entry := map[string]any{
					"kind":     m[1],
					"name":     m[2],
					"line":     i + 1,
					"exported": true,
				}
				if lod >= 3 {
					entry["signature"] = condenseSignature(raw)
				}
				decls = append(decls, entry)
			}
			continue
		}
		leading := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if leading <= 2 {
			if m := tsTopDeclRE.FindStringSubmatch(strings.TrimLeft(raw, " \t")); m != nil {
				declNames = append(declNames, m[2])
				if lod >= 2 {
					entry := map[string]any{
						"kind":     m[1],
						"name":     m[2],
						"line":     i + 1,
						"exported": false,
					}
					if lod >= 3 {
						entry["signature"] = condenseSignature(raw)
					}
					decls = append(decls, entry)
				}
			}
		}
	}

	if lod == 1 {
		out["decl_names"] = declNames
		return out
	}
	out["imports"] = imports
	out["decls"] = decls
	return out
}

func outlineFileTSFromPath(absPath, rel string, lod int) map[string]any {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return map[string]any{"path": rel, "language": tsLangFor(rel), "error": err.Error()}
	}
	return outlineFileTS(rel, string(data), lod)
}

func tsLangFor(p string) string {
	switch filepath.Ext(p) {
	case ".tsx":
		return "tsx"
	case ".ts", ".mts", ".cts":
		return "ts"
	case ".jsx":
		return "jsx"
	case ".js":
		return "js"
	}
	return "ts"
}

func isTSFile(p string) bool {
	switch filepath.Ext(p) {
	case ".ts", ".tsx", ".js", ".jsx", ".mts", ".cts":
		return true
	}
	return false
}

var (
	tsImportSimpleRE    = regexp.MustCompile(`^\s*import\s+(?:type\s+)?(?:[^"';]+?)\s+from\s+["']([^"']+)["']`)
	tsImportSideRE      = regexp.MustCompile(`^\s*import\s+["']([^"']+)["']`)
	tsExportFromRE      = regexp.MustCompile(`^\s*export\s+(?:type\s+)?(?:\*|\{[^}]*\})\s+from\s+["']([^"']+)["']`)
	tsExportDefaultRE   = regexp.MustCompile(`^\s*export\s+default\s+(?:async\s+)?(?:(function|class)\s+(\w+)|(\w+))?`)
	tsExportNamedDeclRE = regexp.MustCompile(`^\s*export\s+(?:default\s+)?(?:abstract\s+)?(?:async\s+)?(function\*?|class|interface|type|enum|const|let|var)\s+(\w+)`)
	tsTopDeclRE         = regexp.MustCompile(`^(?:async\s+)?(function\*?|class|interface|type|enum)\s+(\w+)`)
)

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
	if i := strings.IndexByte(s, '{'); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	const max = 200
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}
