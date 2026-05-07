package mcpcmd

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerGoOutline registers a `go_outline` tool that returns a structured
// summary of every top-level declaration in a Go file or directory. Agents
// should prefer this over read_file when probing what a package exposes —
// it's far smaller and skips function bodies entirely.
func registerGoOutline(srv *mcp.Server, root string) {
	type goOutlineIn struct {
		Path string `json:"path,omitempty" jsonschema:"repo-relative .go file or directory (defaults to repo root)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "go_outline",
		Description: "Parse Go source and return a structured outline: package, imports, top-level type/func/var/const declarations with signatures and doc comments. Function bodies are omitted, so this is much smaller than read_file. Accepts a single .go file or a directory (recursive, skips vendor/, .git/, node_modules/).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in goOutlineIn) (*mcp.CallToolResult, any, error) {
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
					if name == ".git" || name == "node_modules" || name == "vendor" {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(p, ".go") {
					paths = append(paths, p)
				}
				return nil
			})
		} else {
			if !strings.HasSuffix(abs, ".go") {
				return nil, nil, fmt.Errorf("not a .go file: %s", target)
			}
			paths = []string{abs}
		}

		fset := token.NewFileSet()
		var files []map[string]any
		for _, p := range paths {
			rel, _ := filepath.Rel(root, p)
			f, parseErr := parser.ParseFile(fset, p, nil, parser.ParseComments|parser.SkipObjectResolution)
			if parseErr != nil {
				files = append(files, map[string]any{"path": rel, "parse_error": parseErr.Error()})
				continue
			}
			files = append(files, summarizeGoFile(fset, rel, f))
		}
		return nil, map[string]any{"path": target, "files": files}, nil
	})
}

func summarizeGoFile(fset *token.FileSet, rel string, f *ast.File) map[string]any {
	var imports []map[string]any
	for _, im := range f.Imports {
		entry := map[string]any{"path": strings.Trim(im.Path.Value, "\"")}
		if im.Name != nil {
			entry["alias"] = im.Name.Name
		}
		imports = append(imports, entry)
	}

	var decls []map[string]any
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			decls = append(decls, summarizeFunc(fset, decl))
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if entry := summarizeSpec(fset, decl, spec); entry != nil {
					decls = append(decls, entry)
				}
			}
		}
	}
	return map[string]any{
		"path":    rel,
		"package": f.Name.Name,
		"imports": imports,
		"decls":   decls,
	}
}

func summarizeFunc(fset *token.FileSet, d *ast.FuncDecl) map[string]any {
	name := d.Name.Name
	out := map[string]any{
		"kind":     "func",
		"name":     name,
		"line":     fset.Position(d.Pos()).Line,
		"exported": ast.IsExported(name),
	}
	// Reconstruct the signature without the body.
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
	if doc := firstDocLine(d.Doc); doc != "" {
		out["doc"] = doc
	}
	return out
}

func summarizeSpec(fset *token.FileSet, gen *ast.GenDecl, spec ast.Spec) map[string]any {
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
			entry["underlying"] = exprText(fset, s.Type)
		}
		if doc := firstDocLine(s.Doc); doc != "" {
			entry["doc"] = doc
		} else if doc := firstDocLine(gen.Doc); doc != "" {
			entry["doc"] = doc
		}
		return entry
	case *ast.ValueSpec:
		kind := "var"
		if gen.Tok == token.CONST {
			kind = "const"
		}
		// One spec can declare multiple names (var a, b int). Flatten into the
		// first name for outline brevity but record all names in the entry.
		first := s.Names[0]
		names := make([]string, 0, len(s.Names))
		for _, n := range s.Names {
			names = append(names, n.Name)
		}
		entry := map[string]any{
			"kind":     kind,
			"name":     first.Name,
			"line":     fset.Position(s.Pos()).Line,
			"exported": ast.IsExported(first.Name),
		}
		if len(names) > 1 {
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
