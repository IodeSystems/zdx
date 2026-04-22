package devtools

import (
	"sort"
	"testing"

	"github.com/iodesystems/zdx-go/internal/config"
)

func TestParseShard(t *testing.T) {
	cases := []struct {
		in   string
		n, m int
	}{
		{"2/3", 2, 3},
		{"1/1", 1, 1},
		{"3/5", 3, 5},
		// fallback cases
		{"", 1, 1},
		{"foo", 1, 1},
		{"0/2", 1, 1},
		{"3/2", 1, 1},
		{"2/0", 1, 1},
		{"2", 1, 1},
		{"1/", 1, 1},
		{"/3", 1, 1},
	}
	for _, c := range cases {
		n, m := parseShard(c.in)
		if n != c.n || m != c.m {
			t.Errorf("parseShard(%q) = (%d,%d), want (%d,%d)", c.in, n, m, c.n, c.m)
		}
	}
}

func TestShardTests(t *testing.T) {
	for _, size := range []int{0, 1, 5, 7, 13} {
		tests := make([]string, size)
		for i := range tests {
			tests[i] = string(rune('a' + i))
		}
		for _, m := range []int{1, 2, 3, 5} {
			if m > size && size > 0 {
				// some shards will be empty — still valid
			}
			union := map[string]int{}
			for n := 1; n <= m; n++ {
				shard := shardTests(tests, n, m)
				for _, v := range shard {
					union[v]++
				}
				// round-robin contract
				for i, v := range tests {
					inShard := (i%m)+1 == n
					found := false
					for _, sv := range shard {
						if sv == v {
							found = true
							break
						}
					}
					if inShard != found {
						t.Errorf("size=%d m=%d n=%d: item[%d]=%q inShard=%v found=%v", size, m, n, i, v, inShard, found)
					}
				}
			}
			// union = full input, each item exactly once
			for _, v := range tests {
				if union[v] != 1 {
					t.Errorf("size=%d m=%d: item %q in union %d times, want 1", size, m, v, union[v])
				}
			}
		}
	}

	// identity: N=1, M=1
	in := []string{"x", "y", "z"}
	got := shardTests(in, 1, 1)
	if len(got) != len(in) {
		t.Errorf("identity shard: got %v, want %v", got, in)
	}

	// empty input
	if out := shardTests(nil, 1, 2); out != nil {
		t.Errorf("empty input: got %v, want nil", out)
	}
}

func namedSuites(names ...string) []config.NamedSuite {
	out := make([]config.NamedSuite, len(names))
	for i, n := range names {
		out[i] = config.NamedSuite{Name: n}
	}
	return out
}

func suiteNames(ss []config.NamedSuite) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Name
	}
	return out
}

func TestApplyShard(t *testing.T) {
	suites := namedSuites("a", "b", "c", "d", "e")

	for _, m := range []int{1, 2, 3, 5} {
		union := map[string]int{}
		for n := 1; n <= m; n++ {
			spec := string([]byte{byte('0' + n), '/', byte('0' + m)})
			shard := applyShard(suites, spec)
			for _, s := range shard {
				union[s.Name]++
			}
			// round-robin contract
			for i, s := range suites {
				inShard := (i%m)+1 == n
				found := false
				for _, sv := range shard {
					if sv.Name == s.Name {
						found = true
						break
					}
				}
				if inShard != found {
					t.Errorf("m=%d n=%d spec=%q: item[%d]=%q inShard=%v found=%v", m, n, spec, i, s.Name, inShard, found)
				}
			}
		}
		for _, s := range suites {
			if union[s.Name] != 1 {
				t.Errorf("m=%d: item %q in union %d times, want 1", m, s.Name, union[s.Name])
			}
		}
	}

	// invalid specs return the original slice unchanged
	original := namedSuites("x", "y")
	for _, bad := range []string{"", "foo", "0/2", "3/2", "2/0", "2"} {
		got := applyShard(original, bad)
		gotNames := suiteNames(got)
		wantNames := suiteNames(original)
		sort.Strings(gotNames)
		sort.Strings(wantNames)
		for i := range wantNames {
			if i >= len(gotNames) || gotNames[i] != wantNames[i] {
				t.Errorf("applyShard(bad=%q): got %v, want %v", bad, gotNames, wantNames)
				break
			}
		}
	}
}

// TODO: testHarnessRunE reads --shard but discards it (test.go:167-169: `_ = shard`).
// The spec targets the e2e binary path (shardTests via e2eRunArgs), which IS wired up.
// Wire testHarnessRunE to pass shard through to the GoBinAdapter when that path is exercised.
