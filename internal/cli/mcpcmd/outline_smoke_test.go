package mcpcmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOutlineGo_smoke(t *testing.T) {
	root := t.TempDir()
	src := `// Package demo is for testing.
package demo

import (
	"fmt"
	stdjson "encoding/json"
)

// Greet says hello.
func Greet(name string) string {
	return fmt.Sprintf("hi %s", name)
}

type Adder struct {
	Total int
}

func (a *Adder) Add(n int) int {
	a.Total += n
	return a.Total
}

type Sayer interface {
	Say() string
}

const Version = "1.0"

var _ = stdjson.Marshaler(nil)
`
	if err := os.WriteFile(filepath.Join(root, "demo.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cs := setupClient(t, root)
	// LOD 3 = full detail incl. doc lines.
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "outline",
		Arguments: map[string]any{"path": "demo.go", "lod": 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := unmarshalStructured(t, res.StructuredContent)
	files, _ := got["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f0 := files[0].(map[string]any)
	if f0["package"] != "demo" {
		t.Errorf("package=%v want demo", f0["package"])
	}
	if f0["language"] != "go" {
		t.Errorf("language=%v want go", f0["language"])
	}
	imports, _ := f0["imports"].([]any)
	if len(imports) != 2 {
		t.Errorf("imports: got %d want 2", len(imports))
	}
	decls, _ := f0["decls"].([]any)
	if len(decls) < 5 {
		t.Errorf("decls: got %d want >=5: %v", len(decls), decls)
	}
	var greet map[string]any
	for _, d := range decls {
		m := d.(map[string]any)
		if m["name"] == "Greet" {
			greet = m
			break
		}
	}
	if greet == nil {
		t.Fatalf("missing Greet decl in %v", decls)
	}
	if greet["kind"] != "func" {
		t.Errorf("Greet.kind=%v want func", greet["kind"])
	}
	if sig, _ := greet["signature"].(string); sig != "func Greet(name string) string" {
		t.Errorf("Greet.signature=%q", sig)
	}
	if doc, _ := greet["doc"].(string); doc != "Greet says hello." {
		t.Errorf("Greet.doc=%q (LOD 3 should expose doc)", doc)
	}

	// LOD 1: skeleton — decl_names only, no signatures or imports map.
	resLow, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "outline",
		Arguments: map[string]any{"path": "demo.go", "lod": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	gotLow := unmarshalStructured(t, resLow.StructuredContent)
	filesLow, _ := gotLow["files"].([]any)
	if len(filesLow) != 1 {
		t.Fatalf("LOD 1: expected 1 file, got %d", len(filesLow))
	}
	f0Low := filesLow[0].(map[string]any)
	if _, hasDecls := f0Low["decls"]; hasDecls {
		t.Errorf("LOD 1 should not include 'decls', got %v", f0Low)
	}
	if _, hasNames := f0Low["decl_names"]; !hasNames {
		t.Errorf("LOD 1 should include 'decl_names', got %v", f0Low)
	}

	// json_path: navigate into the file's package field.
	resPath, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "outline",
		Arguments: map[string]any{"path": "demo.go", "lod": 2, "json_path": "files/0/package"},
	})
	if err != nil {
		t.Fatal(err)
	}
	gotPath := unmarshalStructured(t, resPath.StructuredContent)
	if gotPath["result"] != "demo" {
		t.Errorf("json_path 'files/0/package': got %v want \"demo\"", gotPath["result"])
	}
}

func TestOutlineTS_smoke(t *testing.T) {
	root := t.TempDir()
	src := `// Test file
import { useState, useEffect } from "react";
import type { FC } from "react";
import * as fs from "node:fs";
import "./side-effect";

export const VERSION = "1.0";

export function add(a: number, b: number): number {
	return a + b;
}

export interface User {
	id: number;
	name: string;
}

export type UserId = number;

export class Service {
	greet() { return "hi"; }
}

export default function App() {
	return null;
}

export * from "./util";

function unexported() { return 1; }
`
	if err := os.WriteFile(filepath.Join(root, "demo.ts"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	cs := setupClient(t, root)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "outline",
		Arguments: map[string]any{"path": "demo.ts", "lod": 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := unmarshalStructured(t, res.StructuredContent)
	files, _ := got["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f0 := files[0].(map[string]any)
	if f0["language"] != "ts" {
		t.Errorf("language=%v want ts", f0["language"])
	}
	imports, _ := f0["imports"].([]any)
	if len(imports) != 4 {
		t.Errorf("imports: got %d want 4: %v", len(imports), imports)
	}
	decls, _ := f0["decls"].([]any)
	wantNames := map[string]bool{"VERSION": false, "add": false, "User": false, "UserId": false, "Service": false, "App": false, "unexported": false}
	for _, d := range decls {
		m := d.(map[string]any)
		if name, _ := m["name"].(string); name != "" {
			if _, ok := wantNames[name]; ok {
				wantNames[name] = true
			}
		}
	}
	for n, found := range wantNames {
		if !found {
			t.Errorf("ts outline missing decl %q in %v", n, decls)
		}
	}
}

func TestOutlineUnsupported_telemetry(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "demo.py"), []byte("def hi():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "demo.rs"), []byte("fn hi() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := setupClient(t, root)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "outline",
		Arguments: map[string]any{"path": ".", "lod": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := unmarshalStructured(t, res.StructuredContent)
	unsup, _ := got["unsupported_extensions"].([]any)
	if len(unsup) != 2 {
		t.Errorf("expected 2 unsupported extensions (.py, .rs), got %v", unsup)
	}
	missLog := filepath.Join(root, ".zdx", "agent", "outline-misses.jsonl")
	if _, err := os.Stat(missLog); err != nil {
		t.Errorf("expected miss log at %s: %v", missLog, err)
	}
}

func TestNavigateJSONPath(t *testing.T) {
	obj := map[string]any{
		"files": []any{
			map[string]any{"path": "a.go", "decls": []any{
				map[string]any{"name": "Foo"},
				map[string]any{"name": "Bar"},
			}},
			map[string]any{"path": "b.go", "decls": []any{
				map[string]any{"name": "Baz"},
			}},
		},
	}
	cases := []struct {
		path string
		want any
	}{
		{"", obj},
		{"files/0/path", "a.go"},
		{"files/1/decls/0/name", "Baz"},
	}
	for _, c := range cases {
		got, err := navigateJSONPath(obj, c.path)
		if err != nil {
			t.Errorf("navigate %q: %v", c.path, err)
			continue
		}
		if got == nil && c.want != nil {
			t.Errorf("navigate %q: got nil want %v", c.path, c.want)
		}
		// Spot-check string results
		if s, ok := c.want.(string); ok {
			if gs, _ := got.(string); gs != s {
				t.Errorf("navigate %q: got %q want %q", c.path, gs, s)
			}
		}
	}
	// Wildcard projection
	got, err := navigateJSONPath(obj, "files/*/path")
	if err != nil {
		t.Fatal(err)
	}
	gs, _ := got.([]any)
	if len(gs) != 2 || gs[0] != "a.go" || gs[1] != "b.go" {
		t.Errorf("'files/*/path': got %v want [a.go, b.go]", gs)
	}
}

func setupClient(t *testing.T, root string) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterFSTools(srv, root)
	RegisterShellTools(srv, root)
	RegisterOutlineTools(srv, root)
	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	c := mcp.NewClient(&mcp.Implementation{Name: "tester", Version: "0"}, nil)
	cs, err := c.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cs
}
