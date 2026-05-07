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
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "go_outline",
		Arguments: map[string]any{"path": "demo.go"},
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
	imports, _ := f0["imports"].([]any)
	if len(imports) != 2 {
		t.Errorf("imports: got %d want 2", len(imports))
	}
	decls, _ := f0["decls"].([]any)
	// expect: Greet (func), Adder (type), Adder.Add (func), Sayer (type), Version (const), _ (var)
	if len(decls) < 5 {
		t.Errorf("decls: got %d want >=5: %v", len(decls), decls)
	}
	// Spot-check Greet
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
		t.Errorf("Greet.doc=%q", doc)
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
		Name:      "ts_outline",
		Arguments: map[string]any{"path": "demo.ts"},
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
			t.Errorf("ts_outline missing decl %q in %v", n, decls)
		}
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
