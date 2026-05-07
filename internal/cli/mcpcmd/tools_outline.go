package mcpcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// outlineBackend produces a structural summary of a single source file at the
// requested level of detail (0..3). See outlineFileGo / outlineFileTS for LOD
// semantics.
type outlineBackend func(absPath, rel string, lod int) map[string]any

var outlineBackends = map[string]outlineBackend{
	".go":  outlineFileGoFromPath,
	".ts":  outlineFileTSFromPath,
	".tsx": outlineFileTSFromPath,
	".js":  outlineFileTSFromPath,
	".jsx": outlineFileTSFromPath,
	".mts": outlineFileTSFromPath,
	".cts": outlineFileTSFromPath,
}

// RegisterOutlineTools registers the unified `outline` tool — one tool that
// dispatches by file extension to the right backend (Go AST or TS regex). For
// extensions without a backend, the request is logged to
// .zdx/agent/outline-misses.jsonl as evidence for which languages to add.
//
// The agent should usually start with lod=1 (decl names only) on a directory
// to see what's there, then drill in with json_path + higher lod on specific
// files. This matches the natural laziness of probing-then-reading and keeps
// per-call payloads small.
func RegisterOutlineTools(srv *mcp.Server, root string) {
	type outlineIn struct {
		Path     string `json:"path,omitempty" jsonschema:"repo-relative file or directory (defaults to repo root)"`
		Lod      int    `json:"lod,omitempty" jsonschema:"level of detail: 0=files-only, 1=decl names, 2=signatures (default), 3=full incl. doc"`
		JSONPath string `json:"json_path,omitempty" jsonschema:"slash-separated navigation into the result (e.g. 'files/0/decls', 'files/*/path'). When set, returns only the navigated subtree."`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name: "outline",
		Description: "Structural outline of source files. Backends: Go (AST), TypeScript/TSX/JS/JSX (regex). " +
			"Pass `lod` (0=files-only, 1=decl names, 2=signatures (default), 3=full with doc lines) to control verbosity, " +
			"and `json_path` (slash-separated, e.g. 'files/0/decls', 'files/*/decls/*/name') to navigate into the result without re-running the call. " +
			"Function bodies are never included. Unsupported extensions are logged to .zdx/agent/outline-misses.jsonl for telemetry; " +
			"prefer this tool over read_file when probing structure.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in outlineIn) (*mcp.CallToolResult, any, error) {
		target := in.Path
		if target == "" {
			target = "."
		}
		lod := in.Lod
		if lod < 0 {
			lod = 0
		}
		if lod > 3 {
			lod = 3
		}
		abs, err := resolveInRoot(root, target)
		if err != nil {
			return nil, nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, nil, err
		}

		var (
			files       []map[string]any
			unsupported = map[string][]string{} // ext -> sample relpaths
		)
		const maxUnsupportedSamplesPerExt = 5
		const maxFiles = 500

		recordUnsupported := func(rel, ext string) {
			samples := unsupported[ext]
			if len(samples) < maxUnsupportedSamplesPerExt {
				unsupported[ext] = append(samples, rel)
			}
		}

		walkOne := func(absPath string) {
			rel, _ := filepath.Rel(root, absPath)
			ext := strings.ToLower(filepath.Ext(absPath))
			backend, ok := outlineBackends[ext]
			if !ok {
				if ext != "" {
					recordUnsupported(rel, ext)
				}
				return
			}
			files = append(files, backend(absPath, rel, lod))
		}

		if info.IsDir() {
			_ = filepath.Walk(abs, func(p string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil || fi == nil {
					return nil
				}
				if fi.IsDir() {
					name := filepath.Base(p)
					if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" {
						return filepath.SkipDir
					}
					return nil
				}
				if len(files) >= maxFiles {
					return filepath.SkipAll
				}
				walkOne(p)
				return nil
			})
		} else {
			walkOne(abs)
		}

		// Telemetry: log unsupported extensions encountered in this request.
		if len(unsupported) > 0 {
			_ = appendOutlineMiss(root, target, unsupported)
		}

		out := map[string]any{
			"path":  target,
			"lod":   lod,
			"files": files,
		}
		if len(unsupported) > 0 {
			summary := make([]map[string]any, 0, len(unsupported))
			for ext, samples := range unsupported {
				summary = append(summary, map[string]any{
					"ext":     ext,
					"samples": samples,
					"count":   len(samples),
				})
			}
			sort.Slice(summary, func(i, j int) bool { return summary[i]["ext"].(string) < summary[j]["ext"].(string) })
			out["unsupported_extensions"] = summary
			out["note"] = "extensions without an outline backend were logged to .zdx/agent/outline-misses.jsonl; for those files, fall back to read_file or grep."
		}

		// json_path navigation: agent drilling into a specific subtree.
		if in.JSONPath != "" {
			sub, navErr := navigateJSONPath(out, in.JSONPath)
			if navErr != nil {
				return nil, nil, fmt.Errorf("json_path %q: %w", in.JSONPath, navErr)
			}
			out = map[string]any{
				"path":      target,
				"lod":       lod,
				"json_path": in.JSONPath,
				"result":    sub,
			}
		}

		// Spillover: if the rendered result still exceeds the soft cap, write
		// the full structure to scratch and return only the head + pointer.
		if buf, jErr := json.Marshal(out); jErr == nil && len(buf) > 32*1024 {
			scratchPath, sErr := writeScratch(root, "outline", string(buf))
			abridged := map[string]any{
				"path":         target,
				"lod":          lod,
				"total_files":  len(files),
				"truncated":    true,
				"hint":         "Outline payload exceeded 32KB. Re-call with lod=0 or lod=1 for a skeleton, or pass json_path to drill into a subtree (e.g. 'files/0', 'files/*/decls').",
				"scratch_path": scratchPath,
			}
			if sErr != nil {
				abridged["scratch_error"] = sErr.Error()
			}
			if in.JSONPath != "" {
				abridged["json_path"] = in.JSONPath
			}
			out = abridged
		}

		return nil, out, nil
	})
}

// appendOutlineMiss writes one JSON line per outline call that hit unsupported
// extensions. Operators inspect the file periodically to decide which
// languages deserve a backend. Telemetry only — never blocks the request.
func appendOutlineMiss(root, requestPath string, unsupported map[string][]string) error {
	dir := filepath.Join(root, ".zdx", "agent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "outline-misses.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	rec := map[string]any{
		"ts":           time.Now().UTC().Format(time.RFC3339Nano),
		"request_path": requestPath,
		"unsupported":  unsupported,
	}
	buf, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = f.Write(append(buf, '\n'))
	return err
}

// navigateJSONPath walks obj following slash-separated steps. Each step is
// either a map key, an integer array index, or "*" which projects remaining
// steps over each element of an array. Empty path returns obj unchanged.
//
// Examples:
//
//	""                        -> obj
//	"files"                   -> obj["files"]
//	"files/0"                 -> obj["files"][0]
//	"files/0/decls/3/name"    -> obj["files"][0]["decls"][3]["name"]
//	"files/*/path"            -> [obj["files"][i]["path"] for each i]
//	"files/*/decls/*/name"    -> nested projection
func navigateJSONPath(obj any, path string) (any, error) {
	if path == "" {
		return obj, nil
	}
	steps := strings.Split(path, "/")
	return navigateSteps(obj, steps)
}

func navigateSteps(cur any, steps []string) (any, error) {
	for i, s := range steps {
		if s == "" {
			continue
		}
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[s]
			if !ok {
				return nil, fmt.Errorf("step %d: key %q not found", i, s)
			}
			cur = next
		case []any:
			next, err := navigateSlice(v, s, i, steps[i+1:])
			if err != nil {
				return nil, err
			}
			if s == "*" {
				return next, nil
			}
			cur = next
		case []map[string]any:
			// Common case: outline produces []map[string]any for files. Box
			// to []any for uniform handling.
			boxed := make([]any, len(v))
			for j, e := range v {
				boxed[j] = e
			}
			next, err := navigateSlice(boxed, s, i, steps[i+1:])
			if err != nil {
				return nil, err
			}
			if s == "*" {
				return next, nil
			}
			cur = next
		default:
			return nil, fmt.Errorf("step %d: cannot navigate into %T at key %q", i, cur, s)
		}
	}
	return cur, nil
}

// navigateSlice resolves one step against a []any. When step is "*", projects
// the remaining tail steps over each element. Otherwise treats step as an int
// index and returns the element.
func navigateSlice(v []any, step string, stepIdx int, tail []string) (any, error) {
	if step == "*" {
		out := make([]any, 0, len(v))
		for _, e := range v {
			sub, err := navigateSteps(e, tail)
			if err == nil {
				out = append(out, sub)
			}
		}
		return out, nil
	}
	idx, err := strconv.Atoi(step)
	if err != nil {
		return nil, fmt.Errorf("step %d: array requires int index or '*', got %q", stepIdx, step)
	}
	if idx < 0 || idx >= len(v) {
		return nil, fmt.Errorf("step %d: index %d out of range [0,%d)", stepIdx, idx, len(v))
	}
	return v[idx], nil
}
