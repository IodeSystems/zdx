package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestIsWorkingTreeDirty covers the IS-1062 clean-tree gate primitive:
// `dx issue close` for impl/ops issues calls this and refuses on a non-empty
// `git status --porcelain` unless --force or --commit=<sha> is passed.
func TestIsWorkingTreeDirty(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "--initial-branch=main")
	run("config", "commit.gpgsign", "false")

	// chdir into the temp repo: isWorkingTreeDirty/gitHEAD shell out to git
	// against the cwd (mirrors how `dx issue close` invokes them from the
	// operator's working directory).
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if dirty, err := isWorkingTreeDirty(); err != nil || dirty {
		t.Fatalf("empty repo: want clean+nil, got dirty=%v err=%v", dirty, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if dirty, err := isWorkingTreeDirty(); err != nil || !dirty {
		t.Fatalf("untracked file: want dirty+nil, got dirty=%v err=%v", dirty, err)
	}

	run("add", "a.txt")
	run("commit", "-m", "init")
	if dirty, err := isWorkingTreeDirty(); err != nil || dirty {
		t.Fatalf("post-commit: want clean+nil, got dirty=%v err=%v", dirty, err)
	}
	if sha := gitHEAD(); sha == "" {
		t.Fatal("gitHEAD post-commit: want non-empty SHA, got empty")
	}
}

func TestDecodeEscapes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"no escapes", "no escapes"},
		{`line1\nline2`, "line1\nline2"},
		{`para1\n\npara2`, "para1\n\npara2"},
		{`tab\there`, "tab\there"},
		{`literal\\nbackslash-n`, `literal\nbackslash-n`},
		{`mixed \n and \\n`, "mixed \n and " + `\n`},
		{`trailing\`, `trailing\`},
		{`\unknown`, `\unknown`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := decodeEscapes(tc.in)
			if got != tc.want {
				t.Fatalf("decodeEscapes(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtractDecompositionCandidates(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty context",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "   \n\t\n",
			want:  nil,
		},
		{
			name:  "no signals",
			input: "Closed as completed. The migration ran cleanly and the rollback path was verified.",
			want:  nil,
		},
		{
			name: "numbered list with task markers",
			input: "Plan:\n" +
				"1. [ ] Wire the gate into issueCloseCmd\n" +
				"2. [ ] Add detector helper\n" +
				"3. [x] Cover with unit tests",
			want: []string{
				"Wire the gate into issueCloseCmd",
				"Add detector helper",
			},
		},
		{
			name: "numbered list plain items no task marker yields no candidates",
			input: "Plan:\n" +
				"1. Wire the gate into issueCloseCmd\n" +
				"2. Add detector helper\n",
			want: nil,
		},
		{
			name: "bulleted list with - and * and task markers",
			input: "Outstanding:\n" +
				"- [ ] Migrate caller A\n" +
				"* [ ] Migrate caller B\n" +
				"  - [ ] Indented child\n",
			want: []string{
				"Migrate caller A",
				"Migrate caller B",
				"Indented child",
			},
		},
		{
			name: "plain bullets without task markers yield no candidates",
			input: "Outstanding:\n" +
				"- Migrate caller A\n" +
				"- Migrate caller B\n",
			want: nil,
		},
		{
			name:  "future-tense without list marker",
			input: "We should rename the helper before the next release.",
			want:  []string{"We should rename the helper before the next release."},
		},
		{
			name: "explicit DECOMPOSITION header captures bare and task-marked list lines",
			input: "Context body unrelated to children.\n" +
				"\n" +
				"## DECOMPOSITION\n" +
				"- [ ] File child A\n" +
				"Bare line counts too\n" +
				"\n" +
				"## NEXT SECTION\n" +
				"Bland prose outside the section.",
			want: []string{
				"File child A",
				"Bare line counts too",
			},
		},
		{
			name: "DECOMPOSITION header without leading hashes",
			input: "DECOMPOSITION\n" +
				"- [ ] one\n" +
				"- [ ] two\n",
			want: []string{"one", "two"},
		},
		{
			name: "dedup duplicate candidates",
			input: "1. [ ] same item\n" +
				"2. [ ] same item\n" +
				"- [ ] same item\n",
			want: []string{"same item"},
		},
		{
			name:  "TODO marker triggers detection",
			input: "TODO: write the doctor rung for this concern",
			want:  []string{"TODO: write the doctor rung for this concern"},
		},
		{
			name: "standard impl template sections suppress extraction",
			input: "WHAT SHOULD HAPPEN\n" +
				"runDecompositionPathGate's error message should suggest a working command. The flag should read --type.\n" +
				"\n" +
				"WHAT DID HAPPEN\n" +
				"Running the command yields 'unknown flag: --issue-type'.\n" +
				"\n" +
				"FIX\n" +
				"  - Before: dx issue edit %s --issue-type=tracker if this is a tracker, not an impl\n" +
				"  - After:  dx issue edit %s --type=tracker if this is a tracker, not an impl\n",
			want: nil,
		},
		{
			name: "exempt section then explicit DECOMPOSITION still extracts children",
			input: "WHAT SHOULD HAPPEN\n" +
				"The thing should work; needs to be fixed.\n" +
				"\n" +
				"## DECOMPOSITION\n" +
				"- [ ] File child A\n" +
				"- [ ] File child B\n",
			want: []string{"File child A", "File child B"},
		},
		{
			name: "non-exempt markdown header exits exempt mode and resumes extraction",
			input: "WHAT SHOULD HAPPEN\n" +
				"Description here, should not flag.\n" +
				"\n" +
				"## OTHER NOTES\n" +
				"- We should ship this next.\n",
			// future-tense signal fires on the full line; list prefix is preserved
			want: []string{"- We should ship this next."},
		},
		{
			name: "exempt headers tolerate trailing decoration",
			input: "## What should happen\n" +
				"It should work.\n" +
				"\n" +
				"## What did happen (IS-610 / IS-616 decomposition)\n" +
				"1. Item one — known issue should not flag\n" +
				"2. Item two\n" +
				"\n" +
				"## Out of scope\n" +
				"- Whether the approach is right.\n",
			want: nil,
		},
		{
			name: "parenthesized letter prefix on exempt headers suppresses extraction",
			input: "(a) What should happen\n" +
				"- The handler should reject empty payloads.\n" +
				"\n" +
				"(b) What did happen\n" +
				"- It accepted the empty payload and crashed.\n" +
				"\n" +
				"(c) Implementation direction\n" +
				"- Add a guard before the call.\n" +
				"\n" +
				"## (a) What should happen\n" +
				"- Markdown-prefixed variant should also suppress.\n",
			want: nil,
		},
		{
			name:  "'todos' noun does not trigger 'todo' future signal",
			input: "## Root cause\nThe gate matches todos and other nouns containing the word as a substring.",
			want:  nil,
		},
		{
			name:  "Examples section suppresses bullet extraction",
			input: "Examples observed:\n- IS-682 alert title string quoted as evidence\n- IS-683 another example\n",
			want:  nil,
		},
		{
			name:  "Examples markdown header suppresses bullet extraction",
			input: "## Examples observed\n- IS-682 alert title string quoted as evidence\n",
			want:  nil,
		},
		{
			name: "code-fenced content is not extracted",
			input: "Here's a diff:\n" +
				"```\n" +
				"- -- Dumped by pg_dump version 18.3\n" +
				"+ -- Dumped by pg_dump version 17.9\n" +
				"```\n" +
				"And a real candidate after the fence:\n" +
				"- [ ] Real follow-up\n",
			want: []string{"Real follow-up"},
		},
		{
			name:  "task-marker unchecked bullet is a candidate",
			input: "- [ ] implement the new handler\n",
			want:  []string{"implement the new handler"},
		},
		{
			name:  "task-marker checked bullet is not a candidate",
			input: "- [x] done thing\n",
			want:  nil,
		},
		{
			name:  "task-marker uppercase X checked bullet is not a candidate",
			input: "- [X] done thing\n",
			want:  nil,
		},
		{
			name: "IS-966 regression: classification-to-branch-role mapping prose bullets are not candidates",
			input: "## Context\n" +
				"Classification mapping:\n" +
				"- library: read-only\n" +
				"- service: deployer\n" +
				"- saas: owner\n",
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractDecompositionCandidates(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("candidates mismatch\n  got:  %#v\n  want: %#v", got, tc.want)
			}
		})
	}
}

func TestParseUnresolvedBranchesError(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "single branch",
			body: `{"title":"Unprocessable Entity","detail":"unresolved_branches: v1.0.x"}`,
			want: []string{"v1.0.x"},
		},
		{
			name: "multiple branches with surrounding prose",
			body: `{"detail":"issue has unresolved_branches: v1.0.x, v1.1.x, dev"}`,
			want: []string{"v1.0.x", "v1.1.x", "dev"},
		},
		{
			name: "unrelated 422",
			body: `{"detail":"IS-1 has 2 open task(s); resolve each before closing"}`,
			want: nil,
		},
		{
			name: "marker without payload",
			body: `{"detail":"unresolved_branches:"}`,
			want: nil,
		},
		{
			name: "garbage body",
			body: `not json`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseUnresolvedBranchesError([]byte(tc.body))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestValidateIssueCloseFlags covers IS-1088: the close-flag taxonomy that
// distinguishes categorical closes (duplicate/wontfix/superseded) from
// force-bypass closes with operator-supplied reasons. See validateIssueCloseFlags.
func TestValidateIssueCloseFlags(t *testing.T) {
	cases := []struct {
		name        string
		reason      string
		note        string
		force       bool
		duplicateOf string
		linkOf      string
		wantErr     string // substring match; "" means no error
	}{
		// IS-1088 force-bypass cases
		{
			name:    "force without reason errors with taxonomy hint",
			force:   true,
			wantErr: "--force requires --reason",
		},
		{
			name:   "force with heuristic-false-positive ok",
			reason: "heuristic-false-positive",
			force:  true,
		},
		{
			name:   "force with emergency ok",
			reason: "emergency",
			force:  true,
		},
		{
			name:   "force with rollback ok",
			reason: "rollback",
			force:  true,
		},
		{
			name:    "force with reason=other but no note errors",
			reason:  "other",
			force:   true,
			wantErr: "--reason=other requires --note",
		},
		{
			name:   "force with reason=other and note ok",
			reason: "other",
			note:   "decomp gate fired on descriptive prose",
			force:  true,
		},
		{
			name:   "force with arbitrary non-empty reason ok (taxonomy is advisory)",
			reason: "incident-2026-05-14",
			force:  true,
		},
		{
			name:   "force with done is also a force-bypass (non-empty reason ok)",
			reason: "done",
			force:  true,
		},
		// Categorical reason cases (IS-1088: keep current behavior)
		{
			name:        "force with duplicate and duplicate-of ok",
			reason:      "duplicate",
			force:       true,
			duplicateOf: "IS-99",
		},
		{
			name:    "duplicate without force errors",
			reason:  "duplicate",
			wantErr: "--reason=duplicate requires --force",
		},
		{
			name:    "wontfix without force errors",
			reason:  "wontfix",
			wantErr: "--reason=wontfix requires --force",
		},
		{
			name:    "superseded without force errors",
			reason:  "superseded",
			wantErr: "--reason=superseded requires --force",
		},
		{
			name:   "force with wontfix ok",
			reason: "wontfix",
			force:  true,
		},
		{
			name:    "duplicate with force but no --duplicate-of errors",
			reason:  "duplicate",
			force:   true,
			wantErr: "--duplicate-of is required",
		},
		{
			name:    "link without --link-of errors",
			reason:  "link",
			wantErr: "--link-of is required",
		},
		{
			name:   "link with --link-of ok (no force needed)",
			reason: "link",
			linkOf: "IS-99",
		},
		// Mutual exclusion + clean-close cases
		{
			name:        "duplicate-of and link-of mutually exclusive",
			reason:      "link",
			linkOf:      "IS-1",
			duplicateOf: "IS-2",
			wantErr:     "mutually exclusive",
		},
		{
			name:   "clean close with reason=done ok",
			reason: "done",
		},
		{
			name: "clean close with no reason ok",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIssueCloseFlags(tc.reason, tc.note, tc.force, tc.duplicateOf, tc.linkOf)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestExtractDecompositionCandidates_CapsAt20(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&b, "- [ ] item %d\n", i)
	}
	got := extractDecompositionCandidates(b.String())
	if len(got) != decompCandidateCap {
		t.Fatalf("want %d candidates (cap), got %d", decompCandidateCap, len(got))
	}
	if got[0] != "item 0" || got[decompCandidateCap-1] != fmt.Sprintf("item %d", decompCandidateCap-1) {
		t.Fatalf("unexpected ordering: first=%q last=%q", got[0], got[len(got)-1])
	}
}
