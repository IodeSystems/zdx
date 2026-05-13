package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// SpinLockInfo describes a tripped spin-lock — N consecutive identical
// (tool_name, args_digest) tool_calls. Carried from the dispatch loop back
// to RunLifecycle via AbortReporter and forwarded to the server's
// close-agent-session payload (TK-1791) so reviewer (IS-1172) and the
// churn-hint pipeline get a structured handle on the abort.
type SpinLockInfo struct {
	Tool        string
	ArgsDigest  string
	RepeatCount int32
	LastTurn    int32
}

// AbortInfo carries structured abort metadata from an adapter to RunLifecycle.
// Reason is empty when the session ended cleanly; spin_lock is currently the
// only populated reason taxonomy (more may join later).
type AbortInfo struct {
	Reason   string
	SpinLock *SpinLockInfo
}

// AbortReporter is an optional adapter interface. RunLifecycle type-asserts
// against it post-Wait and, when present, forwards the structured reason to
// close-agent-session. Adapters that never abort can simply not implement it.
type AbortReporter interface {
	AbortInfo() *AbortInfo
}

// argsDigest returns sha256(canonical_json(args)) so identical-but-
// reordered argument payloads collapse to the same digest. Falls back to a
// digest of the raw bytes when the payload doesn't unmarshal — still gives
// stable equality on bit-identical retries, which is the spin-lock signal.
func argsDigest(argsJSON string) string {
	canon, err := canonicalJSON([]byte(argsJSON))
	if err != nil {
		canon = []byte(argsJSON)
	}
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:])
}

// canonicalJSON re-serializes raw with map keys sorted recursively. Numbers
// keep Go's default float64 encoding from encoding/json, which is fine for
// equality (two identical raw inputs decode to identical floats).
func canonicalJSON(raw []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return marshalCanonical(v)
}

func marshalCanonical(v any) ([]byte, error) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := []byte{'{'}
		for i, k := range keys {
			if i > 0 {
				out = append(out, ',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			out = append(out, kb...)
			out = append(out, ':')
			vb, err := marshalCanonical(t[k])
			if err != nil {
				return nil, err
			}
			out = append(out, vb...)
		}
		out = append(out, '}')
		return out, nil
	case []any:
		out := []byte{'['}
		for i, e := range t {
			if i > 0 {
				out = append(out, ',')
			}
			eb, err := marshalCanonical(e)
			if err != nil {
				return nil, err
			}
			out = append(out, eb...)
		}
		out = append(out, ']')
		return out, nil
	default:
		return json.Marshal(v)
	}
}

// spinTracker holds the last `threshold` (tool, digest) entries; Tripped()
// reports whether all of them are identical and returns the matching entry.
// Threshold ≤ 0 means "disabled" — Push is a no-op and Tripped returns
// (zero, 0, false) so the caller's hot path is zero-cost.
type spinTracker struct {
	threshold int
	entries   []spinEntry
}

type spinEntry struct {
	tool   string
	digest string
}

func newSpinTracker(threshold int) *spinTracker {
	return &spinTracker{threshold: threshold}
}

func (s *spinTracker) enabled() bool { return s.threshold > 0 }

// Push appends one (tool, digest) and trims the buffer to threshold entries.
// No-op when disabled.
func (s *spinTracker) Push(tool, digest string) {
	if !s.enabled() {
		return
	}
	s.entries = append(s.entries, spinEntry{tool: tool, digest: digest})
	if over := len(s.entries) - s.threshold; over > 0 {
		s.entries = s.entries[over:]
	}
}

// Tripped returns the matching entry and the run length when the buffer is
// full and every entry shares the same (tool, digest). The returned count
// equals threshold by construction; callers passing it through to telemetry
// see the exact run that tripped the abort.
func (s *spinTracker) Tripped() (spinEntry, int, bool) {
	if !s.enabled() || len(s.entries) < s.threshold {
		return spinEntry{}, 0, false
	}
	first := s.entries[0]
	for _, e := range s.entries[1:] {
		if e.tool != first.tool || e.digest != first.digest {
			return spinEntry{}, 0, false
		}
	}
	return first, len(s.entries), true
}
