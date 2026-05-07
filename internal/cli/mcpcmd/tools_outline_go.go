package mcpcmd

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
)

// outlineFileGo parses a Go source file and returns a structured outline at
// the requested level of detail. Level semantics:
//
//	0 — {path, language}                          (skeleton: just identifies the file)
//	1 — +package, decl_names                      (one-liner per decl)
//	2 — +imports, decl[].kind/signature/line/exported  (default; decision-grade)
//	3 — +decl[].doc, +var/const names/type, +alias underlying  (full detail)
//
// Function bodies are never included.
func outlineFileGo(rel, content string, lod int) map[string]any {
	out := map[string]any{
		"path":     rel,
		"language": "go",
	}
	if lod < 1 {
		return out
	}

	fset := token.NewFileSet()
	f, parseErr := parser.ParseFile(fset, rel, content, parser.ParseComments|parser.SkipObjectResolution)
	if parseErr != nil {
		out["parse_error"] = parseErr.Error()
		return out
	}
	out["package"] = f.Name.Name

	if lod == 1 {
		// Just decl names — fastest skeleton beyond LOD 0.
		var names []string
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				names = append(names, decl.Name.Name)
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						names = append(names, s.Name.Name)
					case *ast.ValueSpec:
						for _, n := range s.Names {
							names = append(names, n.Name)
						}
					}
				}
			}
		}
		out["decl_names"] = names
		return out
	}

	// LOD 2+: full(ish) decl entries.
	var imports []map[string]any
	for _, im := range f.Imports {
		entry := map[string]any{"path": strings.Trim(im.Path.Value, "\"")}
		if im.Name != nil {
			entry["alias"] = im.Name.Name
		}
		imports = append(imports, entry)
	}
	out["imports"] = imports

	var decls []map[string]any
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			decls = append(decls, summarizeFunc(fset, decl, lod))
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if entry := summarizeSpec(fset, decl, spec, lod); entry != nil {
					decls = append(decls, entry)
				}
			}
		}
	}
	out["decls"] = decls
	return out
}

// outlineFileGoFromPath is a convenience wrapper that reads the file off disk.
func outlineFileGoFromPath(absPath, rel string, lod int) map[string]any {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return map[string]any{"path": rel, "language": "go", "error": err.Error()}
	}
	return outlineFileGo(rel, string(data), lod)
}

func summarizeFunc(fset *token.FileSet, d *ast.FuncDecl, lod int) map[string]any {
	name := d.Name.Name
	out := map[string]any{
		"kind":     "func",
		"name":     name,
		"line":     fset.Position(d.Pos()).Line,
		"exported": ast.IsExported(name),
	}
	var sig strings.Builder
	sig.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		sig.WriteString("(")
		writeFieldList(&sig, fset, d.Recv)
		sig.WriteString(") ")
	}
	sig.WriteString(name)
	if d.Type.TypeParams != nil {
		sig.WriteString("[")
		writeFieldList(&sig, fset, d.Type.TypeParams)
		sig.WriteString("]")
	}
	sig.WriteString("(")
	writeFieldList(&sig, fset, d.Type.Params)
	sig.WriteString(")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		sig.WriteString(" ")
		multi := len(d.Type.Results.List) > 1 || hasNamedFields(d.Type.Results)
		if multi {
			sig.WriteString("(")
		}
		writeFieldList(&sig, fset, d.Type.Results)
		if multi {
			sig.WriteString(")")
		}
	}
	out["signature"] = sig.String()
	if lod >= 3 {
		if doc := firstDocLine(d.Doc); doc != "" {
			out["doc"] = doc
		}
	}
	return out
}

func summarizeSpec(fset *token.FileSet, gen *ast.GenDecl, spec ast.Spec, lod int) map[string]any {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		entry := map[string]any{
			"kind":     "type",
			"name":     s.Name.Name,
			"line":     fset.Position(s.Pos()).Line,
			"exported": ast.IsExported(s.Name.Name),
		}
		switch s.Type.(type) {
		case *ast.StructType:
			entry["type_kind"] = "struct"
		case *ast.InterfaceType:
			entry["type_kind"] = "interface"
		default:
			entry["type_kind"] = "alias"
			if lod >= 3 {
				entry["underlying"] = exprText(fset, s.Type)
			}
		}
		if lod >= 3 {
			if doc := firstDocLine(s.Doc); doc != "" {
				entry["doc"] = doc
			} else if doc := firstDocLine(gen.Doc); doc != "" {
				entry["doc"] = doc
			}
		}
		return entry
	case *ast.ValueSpec:
		kind := "var"
		if gen.Tok == token.CONST {
			kind = "const"
		}
		first := s.Names[0]
		entry := map[string]any{
			"kind":     kind,
			"name":     first.Name,
			"line":     fset.Position(s.Pos()).Line,
			"exported": ast.IsExported(first.Name),
		}
		if lod >= 3 {
			if len(s.Names) > 1 {
				names := make([]string, 0, len(s.Names))
				for _, n := range s.Names {
					names = append(names, n.Name)
				}
				entry["names"] = names
			}
			if s.Type != nil {
				entry["type"] = exprText(fset, s.Type)
			}
			if doc := firstDocLine(s.Doc); doc != "" {
				entry["doc"] = doc
			} else if doc := firstDocLine(gen.Doc); doc != "" {
				entry["doc"] = doc
			}
		}
		return entry
	}
	return nil
}

func writeFieldList(sb *strings.Builder, fset *token.FileSet, fl *ast.FieldList) {
	if fl == nil {
		return
	}
	for i, field := range fl.List {
		if i > 0 {
			sb.WriteString(", ")
		}
		for j, n := range field.Names {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(n.Name)
		}
		if len(field.Names) > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(exprText(fset, field.Type))
	}
}

func hasNamedFields(fl *ast.FieldList) bool {
	if fl == nil {
		return false
	}
	for _, f := range fl.List {
		if len(f.Names) > 0 {
			return true
		}
	}
	return false
}

func exprText(fset *token.FileSet, e ast.Expr) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, e); err != nil {
		return "?"
	}
	return b.String()
}

func firstDocLine(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	t := strings.TrimSpace(g.Text())
	if t == "" {
		return ""
	}
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}
