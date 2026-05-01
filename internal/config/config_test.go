package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestShipValidation(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string // substring; "" means expect success
	}{
		{
			name: "valid round-trip",
			yaml: `
components:
  server:
    ship:
      strategy: simple
      stages:
        - { name: build, run: "go build ./..." }
        - { name: deploy, run: "./bin/deploy" }
`,
		},
		{
			name: "unknown strategy",
			yaml: `
components:
  server:
    ship:
      strategy: yolo
      stages:
        - { name: build, run: "go build" }
`,
			wantErr: "unknown strategy",
		},
		{
			name: "duplicate stage name",
			yaml: `
components:
  server:
    ship:
      strategy: simple
      stages:
        - { name: build, run: "go build" }
        - { name: build, run: "go vet" }
`,
			wantErr: "duplicate stage name",
		},
		{
			name: "empty stages",
			yaml: `
components:
  server:
    ship:
      strategy: simple
`,
			wantErr: "must declare at least one stage",
		},
		{
			name: "missing run",
			yaml: `
components:
  server:
    ship:
      strategy: simple
      stages:
        - { name: build, run: "go build" }
        - { name: deploy }
`,
			wantErr: "missing run command",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			if err := yaml.Unmarshal([]byte(tc.yaml), &cfg); err != nil {
				t.Fatalf("yaml.Unmarshal: %v", err)
			}
			err := cfg.validate()
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("validate() unexpected error: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("validate() expected error containing %q, got nil", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("validate() error = %q, want containing %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestShipValidation_NoShipBlock(t *testing.T) {
	// Components without a ship: section must validate clean — Ship is opt-in.
	const src = `
components:
  server:
    build:
      steps:
        - { name: compile, run: "go build" }
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("validate() ship-less config: %v", err)
	}
}

func TestIsGlobalMode(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"0":     false,
		"no":    false,
		"false": false,
		"1":     true,
		"true":  true,
		"TRUE":  true,
		" 1 ":   true,
	}
	for val, want := range cases {
		t.Setenv("DX_GLOBAL", val)
		if got := IsGlobalMode(); got != want {
			t.Errorf("DX_GLOBAL=%q: got %v, want %v", val, got, want)
		}
	}
}
