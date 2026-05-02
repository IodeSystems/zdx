package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type Route struct {
	Pattern string `json:"pattern"`
	File    string `json:"file"`
	Dynamic bool   `json:"dynamic"`
}

func main() {
	routeTree := flag.String("routeTree", "ui/src/routeTree.gen.ts", "path to routeTree.gen.ts")
	out := flag.String("out", "", "output file (default: stdout)")
	flag.Parse()

	routes, err := extractRoutes(*routeTree)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')

	if *out == "" {
		os.Stdout.Write(data)
	} else {
		if err := os.WriteFile(*out, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", *out, err)
			os.Exit(1)
		}
	}
}

func extractRoutes(routeTreePath string) ([]Route, error) {
	f, err := os.Open(routeTreePath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", routeTreePath, err)
	}
	defer f.Close()

	// Pass 1: build import map — "FooBarRouteImport" -> "ui/src/routes/foo/bar.tsx"
	imports := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "import { Route as ") {
			continue
		}
		// import { Route as AdminIndexRouteImport } from './routes/admin/index'
		rest := line[len("import { Route as "):]
		spaceIdx := strings.IndexByte(rest, ' ')
		if spaceIdx < 0 {
			continue
		}
		name := rest[:spaceIdx]
		fromIdx := strings.Index(line, " from './routes/")
		if fromIdx < 0 {
			continue
		}
		pathPart := line[fromIdx+len(" from './routes/"):]
		closeQuote := strings.IndexByte(pathPart, '\'')
		if closeQuote < 0 {
			continue
		}
		imports[name] = "ui/src/routes/" + pathPart[:closeQuote] + ".tsx"
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan pass 1: %w", err)
	}

	// Pass 2: parse FileRoutesByPath interface
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}
	scanner = bufio.NewScanner(f)

	var routes []Route
	inBlock := false
	depth := 0
	var currentPattern, currentImport string

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !inBlock {
			if trimmed == "interface FileRoutesByPath {" {
				inBlock = true
				depth = 1
			}
			continue
		}

		// At depth 1: detect entry key  '/some/path': {
		if depth == 1 && strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "': {") {
			currentPattern = trimmed[1 : len(trimmed)-4]
		}

		// Capture the import name from preLoaderRoute line
		if strings.HasPrefix(trimmed, "preLoaderRoute: typeof ") {
			currentImport = strings.TrimPrefix(trimmed, "preLoaderRoute: typeof ")
		}

		// Update brace depth
		depth += strings.Count(line, "{") - strings.Count(line, "}")

		// Closing brace returns us to depth 1 — entry complete
		if depth == 1 && currentPattern != "" && currentImport != "" {
			file := imports[currentImport]
			routes = append(routes, Route{
				Pattern: currentPattern,
				File:    file,
				Dynamic: strings.Contains(currentPattern, "$"),
			})
			currentPattern = ""
			currentImport = ""
		}

		if depth == 0 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan pass 2: %w", err)
	}

	return routes, nil
}
