package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPristineBoundary_KeepsLastKAssistants(t *testing.T) {
	events := []SessionEvent{
		{UUID: "u1", Type: "user"},
		{UUID: "a1", Type: "assistant"},
		{UUID: "t1", Type: "user"}, // tool_result
		{UUID: "a2", Type: "assistant"},
		{UUID: "t2", Type: "user"},
		{UUID: "a3", Type: "assistant"},
		{UUID: "t3", Type: "user"},
		{UUID: "a4", Type: "assistant"},
		{UUID: "t4", Type: "user"},
		{UUID: "a5", Type: "assistant"},
	}
	got := pristineBoundary(events, 4)
	// 4 assistants from the end: a5, a4, a3, a2 → boundary at a2 (index 3).
	if got != 3 {
		t.Errorf("expected boundary 3, got %d", got)
	}
}

func TestPristineBoundary_TooFewAssistants(t *testing.T) {
	events := []SessionEvent{
		{UUID: "u1", Type: "user"},
		{UUID: "a1", Type: "assistant"},
	}
	got := pristineBoundary(events, 4)
	if got != 0 {
		t.Errorf("expected whole log pristine when assistants < K, got %d", got)
	}
}

func TestTombstone10pct_SkipsSmallContent(t *testing.T) {
	small := strings.Repeat("x", 100)
	if tombstone10pct(small) != small {
		t.Error("small content should pass through untouched")
	}
}

func TestTombstone10pct_WrapsLargeContent(t *testing.T) {
	big := strings.Repeat("abcdefghij", 1000) // 10_000 bytes
	got := tombstone10pct(big)
	if !strings.Contains(got, `"_tombstoned":true`) {
		t.Errorf("expected tombstone marker, got: %.200s", got)
	}
	if !strings.Contains(got, `"_original_size":10000`) {
		t.Errorf("expected original_size in marker, got: %.200s", got)
	}
	if !strings.Contains(got, `"_hint"`) {
		t.Errorf("expected hint in marker, got: %.200s", got)
	}
	if len(got) >= len(big) {
		t.Errorf("tombstone (%d) should be smaller than original (%d)", len(got), len(big))
	}
}

// fixtureEvents builds a mock session log that exercises:
//   - the original user task event (string content)
//   - one assistant event with a tool_use
//   - one tool_result event (large content)
//   - several more assistant + tool_result pairs to push the first
//     tool_result outside the pristine tail
func fixtureEvents() []SessionEvent {
	return []SessionEvent{
		{
			UUID: "u-task",
			Type: "user",
			Message: map[string]any{
				"role":    "user",
				"content": "do the task",
			},
		},
		{
			UUID: "a-1",
			Type: "assistant",
			Message: map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{
						"type":  "tool_use",
						"id":    "call-1",
						"name":  "run_bash",
						"input": map[string]any{"cmd": "ls"},
					},
				},
			},
		},
		{
			UUID: "tr-1",
			Type: "user",
			Message: map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type":        "tool_result",
						"tool_use_id": "call-1",
						"content":     strings.Repeat("BIG", 4000),
						"is_error":    false,
					},
				},
			},
		},
		// Pad with additional turns so tr-1 falls outside the tail of K=2.
		{UUID: "a-2", Type: "assistant", Message: map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "ok"}}}},
		{UUID: "a-3", Type: "assistant", Message: map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "text", "text": "still working"}}}},
	}
}

func TestHydrate_PristineEmitsAllVerbatim(t *testing.T) {
	events := fixtureEvents()
	msgs := hydrate(events, "sys", "do the task", HydrationStrategy{KTurns: 4, PristineOnly: true})

	// system + originalUser + (assistant tool_use, tool_result, assistant text, assistant text)
	if len(msgs) < 4 {
		t.Fatalf("expected at least 4 hydrated msgs, got %d", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Errorf("expected system+user header, got %q %q", msgs[0].Role, msgs[1].Role)
	}
	// Find the tool_result: must contain the BIG payload verbatim.
	found := false
	for _, m := range msgs {
		if m.Role == "tool" && strings.Contains(m.Content, "BIG") && len(m.Content) > 1000 {
			found = true
		}
	}
	if !found {
		t.Error("expected verbatim tool_result content in pristine hydration")
	}
}

func TestHydrate_TombstoneAppliedReplacesOldToolResult(t *testing.T) {
	events := fixtureEvents()
	// Attach a tombstone for the first tool_result (tr-1).
	events = append(events, SessionEvent{
		Type:             "tombstone.applied",
		EventID:          "tr-1",
		Strategy:         StrategyTombstone10pct,
		CompactedContent: `{"_tombstoned":true,"_original_size":12000,"_partial_content":"BBB","_hint":"rerun"}`,
		OriginalSize:     12000,
	})

	// K=2: keeps a-2, a-3 + their preceding context pristine; tr-1 is outside.
	msgs := hydrate(events, "sys", "do the task", HydrationStrategy{KTurns: 2})

	var tool *string
	for i, m := range msgs {
		if m.Role == "tool" && m.ToolCallID == "call-1" {
			tool = &msgs[i].Content
		}
	}
	if tool == nil {
		t.Fatal("expected tool message for call-1 in hydrated msgs")
	}
	if !strings.Contains(*tool, "_tombstoned") {
		t.Errorf("expected tombstoned content for tr-1, got: %.200s", *tool)
	}
}

func TestHydrate_MostRecentCompactionWins(t *testing.T) {
	events := fixtureEvents()
	events = append(events, SessionEvent{
		Type:             "tombstone.applied",
		EventID:          "tr-1",
		Strategy:         StrategyTombstone10pct,
		CompactedContent: "FIRST",
	})
	events = append(events, SessionEvent{
		Type:             "tombstone.applied",
		EventID:          "tr-1",
		Strategy:         StrategyTombstoneJSONWalk,
		CompactedContent: "SECOND",
	})

	msgs := hydrate(events, "", "do the task", HydrationStrategy{KTurns: 2})
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolCallID == "call-1" {
			if m.Content != "SECOND" {
				t.Errorf("expected most recent compaction to win, got %q", m.Content)
			}
			return
		}
	}
	t.Fatal("did not find tool message for call-1")
}

func TestHydrate_AllowedStrategiesFilter(t *testing.T) {
	events := fixtureEvents()
	events = append(events, SessionEvent{
		Type:             "tombstone.applied",
		EventID:          "tr-1",
		Strategy:         StrategyTombstoneJSONWalk,
		CompactedContent: "JSON_WALK",
	})

	// Only allow tombstone-10pct; the json-walk record above must be ignored
	// and the original (verbatim) content used instead.
	msgs := hydrate(events, "", "do the task", HydrationStrategy{
		KTurns:            2,
		AllowedStrategies: map[string]bool{StrategyTombstone10pct: true},
	})
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolCallID == "call-1" {
			if strings.Contains(m.Content, "JSON_WALK") {
				t.Errorf("strategy filter should have excluded json-walk; got %.200s", m.Content)
			}
			return
		}
	}
	t.Fatal("did not find tool message for call-1")
}

func TestHydrate_SummaryRecordEmittedAfterUserTask(t *testing.T) {
	events := fixtureEvents()
	events = append(events, SessionEvent{
		Type:             "compaction.applied",
		Strategy:         StrategyCompactionSummary,
		CompactedContent: "CONTEXT: did stuff\nNEXT STEPS: keep going",
	})
	// Mark a-1 + tr-1 as rolled into the summary.
	events = append(events,
		SessionEvent{Type: "compaction.applied", EventID: "a-1", Strategy: StrategyRolledIntoSummary},
		SessionEvent{Type: "compaction.applied", EventID: "tr-1", Strategy: StrategyRolledIntoSummary},
	)

	msgs := hydrate(events, "sys", "do the task", HydrationStrategy{KTurns: 2})

	if len(msgs) < 3 {
		t.Fatalf("expected system + user + summary at the top, got %d msgs", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[1].Role != "user" || msgs[2].Role != "assistant" {
		t.Fatalf("unexpected header roles: %q %q %q", msgs[0].Role, msgs[1].Role, msgs[2].Role)
	}
	if !strings.Contains(msgs[2].Content, "CONTEXT:") {
		t.Errorf("expected summary content at index 2, got %q", msgs[2].Content)
	}
	// Rolled-into events must not appear.
	for _, m := range msgs {
		if m.Role == "tool" && m.ToolCallID == "call-1" {
			t.Error("rolled-into-summary tool_result should not be emitted")
		}
	}
}

func TestApplyTombstone_ProducesRecordsForLargeOldToolResults(t *testing.T) {
	events := fixtureEvents()
	records := applyTombstone(events, HydrationStrategy{KTurns: 2})
	if len(records) == 0 {
		t.Fatal("expected at least one tombstone record for the large tool_result")
	}
	for _, r := range records {
		if r.Type != "tombstone.applied" {
			t.Errorf("unexpected record type: %q", r.Type)
		}
		if r.Strategy != StrategyTombstone10pct {
			t.Errorf("unexpected strategy: %q", r.Strategy)
		}
		if r.EventID == "" {
			t.Error("tombstone record must reference an event_id")
		}
		if r.CompactedContent == "" {
			t.Error("tombstone record must carry compacted_content")
		}
	}
}

func TestApplyTombstone_SkipsAlreadyCompacted(t *testing.T) {
	events := fixtureEvents()
	events = append(events, SessionEvent{
		Type:             "tombstone.applied",
		EventID:          "tr-1",
		Strategy:         StrategyTombstone10pct,
		CompactedContent: "EXISTING",
	})
	records := applyTombstone(events, HydrationStrategy{KTurns: 2})
	for _, r := range records {
		if r.EventID == "tr-1" {
			t.Error("applyTombstone should not re-compact an event that already has a record")
		}
	}
}

func TestCheckThreshold_BelowSixtyPctStays(t *testing.T) {
	// 50% pressure (4096 reserve + 0 prompt vs 16384 nCtx → 25% used).
	got := CheckThreshold(0, 4096, 16384, PhaseRaw)
	if got != PhaseRaw {
		t.Errorf("expected PhaseRaw at low pressure, got %d", got)
	}
}

func TestCheckThreshold_SixtyPctAdvancesOne(t *testing.T) {
	// 70% used: prompt=8000, reserve=4096, nCtx=16384 → 12096/16384 ≈ 0.738
	if got := CheckThreshold(8000, 4096, 16384, PhaseRaw); got != PhaseTombstone {
		t.Errorf("PhaseRaw at 70%% should advance to PhaseTombstone, got %d", got)
	}
	if got := CheckThreshold(8000, 4096, 16384, PhaseTombstone); got != PhaseCompact {
		t.Errorf("PhaseTombstone at 70%% should advance to PhaseCompact, got %d", got)
	}
}

func TestCheckThreshold_EightyPctReturnsPanic(t *testing.T) {
	// 90% used: prompt=10752, reserve=4096, nCtx=16384 → 14848/16384 ≈ 0.906
	cases := []Phase{PhaseRaw, PhaseTombstone, PhaseCompact}
	for _, current := range cases {
		got := CheckThreshold(10752, 4096, 16384, current)
		if got != PhasePanic {
			t.Errorf("at >80%% from phase %d, expected PhasePanic, got %d", current, got)
		}
	}
}

func TestCheckThreshold_DisabledWhenNCtxUnknown(t *testing.T) {
	got := CheckThreshold(99999, 4096, 0, PhaseTombstone)
	if got != PhaseTombstone {
		t.Errorf("nCtx=0 should leave current phase untouched, got %d", got)
	}
}

func TestCheckThreshold_CompactPhaseStaysAtSixtyPct(t *testing.T) {
	// Already at PhaseCompact and only the 60% trigger fires — there is
	// no further phase to advance to, so we hold steady (panic is the only
	// way out of PhaseCompact).
	got := CheckThreshold(8000, 4096, 16384, PhaseCompact)
	if got != PhaseCompact {
		t.Errorf("PhaseCompact at 60%% should hold, got %d", got)
	}
}

func TestTombstoneJSONWalk_PreservesKeysTruncatesLeaves(t *testing.T) {
	// Build a JSON payload large enough to clear tombstoneMinBytes.
	leaf := strings.Repeat("z", 1000)
	payload := map[string]any{
		"status": "ok",
		"items": []any{
			map[string]any{"id": "a", "body": leaf},
			map[string]any{"id": "b", "body": leaf},
		},
	}
	raw, _ := json.Marshal(payload)
	// Pad to ensure we exceed tombstoneMinBytes.
	for len(raw) < tombstoneMinBytes {
		payload["pad"] = strings.Repeat("p", tombstoneMinBytes)
		raw, _ = json.Marshal(payload)
	}

	out := tombstoneJSONWalk(string(raw))
	if !strings.Contains(out, `"_tombstoned":true`) {
		t.Fatalf("expected tombstone marker, got: %.300s", out)
	}
	if !strings.Contains(out, `"_strategy":"tombstone-json-walk"`) {
		t.Errorf("expected strategy field, got: %.300s", out)
	}
	// Keys from the original payload must survive.
	for _, key := range []string{"status", "items", "id", "body"} {
		if !strings.Contains(out, `"`+key+`"`) {
			t.Errorf("expected key %q to be preserved, got: %.300s", key, out)
		}
	}
	// Long leaf string must be truncated (1000 z's must NOT appear verbatim).
	if strings.Contains(out, leaf) {
		t.Error("expected long leaf string to be truncated by walkAndTruncate")
	}
}

func TestTombstoneJSONWalk_FallsBackForNonJSON(t *testing.T) {
	// Non-JSON input larger than the min-byte floor should fall through
	// to the 10pct tombstone instead of erroring.
	garbage := strings.Repeat("not json at all ", 500)
	out := tombstoneJSONWalk(garbage)
	if !strings.Contains(out, `"_tombstoned":true`) {
		t.Errorf("expected fallback tombstone marker, got: %.200s", out)
	}
	if strings.Contains(out, `"_strategy":"tombstone-json-walk"`) {
		t.Errorf("non-JSON should fall back to tombstone10pct (no _strategy field), got: %.200s", out)
	}
}

func TestTombstoneJSONWalk_SkipsSmallContent(t *testing.T) {
	small := `{"k":"v"}`
	if got := tombstoneJSONWalk(small); got != small {
		t.Errorf("small content should pass through, got %q", got)
	}
}
