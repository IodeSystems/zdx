package agent

// Three-phase context-compaction ladder for the openai chat session loop.
//
// Phase 1 (raw):       msgs[] is just the JSONL session log replayed verbatim.
// Phase 2 (tombstone): large tool_result payloads outside the pristine tail
//                      are replaced with a small structured marker that
//                      tells the agent it can rerun the tool to recover.
// Phase 3 (compact):   the dropped middle is replaced with one synthesized
//                      summary message anchored on the verbatim last
//                      assistant + tool_result.
//
// Original event content is INSERT-only — compactions are ATTACHED as
// separate JSONL records (event types tombstone.applied / compaction.applied)
// so multiple strategies can produce different compacted forms for the same
// event without ever rewriting the original. Hydration picks the most-recent
// matching record per event.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/iodesystems/zdx-go/internal/llm"
)

// Tunables — kept as constants rather than config to keep the surface narrow
// for the first ship. n_ctx is read from the model's /props endpoint at
// session start; the percentage thresholds are applied at every turn.
const (
	defaultPristineTailTurns = 4
	tombstoneFirstFraction   = 0.10 // first 10% of bytes preserved verbatim
	tombstoneMinBytes        = 4096 // skip tombstoning anything already small

	// Trigger thresholds for phase transitions (fraction of n_ctx).
	thresholdEnterPhase2 = 0.60
	thresholdEnterPhase3 = 0.60
	thresholdPanic       = 0.80

	// Compaction-summary length cap (rough character cap, not tokens).
	compactionSummaryCharCap = 4096

	// Default n_ctx when /props is unavailable (matches llama.cpp default).
	defaultNCtx = 8192

	// Estimated reservation for the next response when computing pressure.
	defaultResponseReserveTokens = 4096
)

// Compaction strategy identifiers. Each compaction.applied / tombstone.applied
// event in the JSONL log carries one of these in its `strategy` field.
const (
	StrategyTombstone10pct    = "tombstone-10pct"
	StrategyTombstoneJSONWalk = "tombstone-json-walk"
	StrategyCompactionSummary = "compaction-summary"
	StrategyRolledIntoSummary = "rolled-into-summary"
)

// Phase is the compaction phase recommended by CheckThreshold. Numerically
// 1-indexed so it matches the cs.phase int used by the session loop and
// preserves the "advance by one" ratchet semantics: PhaseRaw → PhaseTombstone
// → PhaseCompact. PhasePanic is a sentinel used when pressure exceeds the
// hard threshold and the caller should advance immediately rather than wait
// for another turn.
type Phase int

const (
	PhaseRaw       Phase = 1 // session log replayed verbatim, no compaction
	PhaseTombstone Phase = 2 // large tool_results outside pristine tail tombstoned
	PhaseCompact   Phase = 3 // dropped middle replaced with synthesized summary
	PhasePanic     Phase = 4 // hard threshold breached; force-advance
)

// CheckThreshold returns the recommended phase based on token pressure
// (promptTokens + responseReserveTokens against nCtx). Mapping:
//
//	usage > 80% × nCtx → PhasePanic (force-advance from any phase)
//	usage > 60% × nCtx → next phase up from current (Raw→Tombstone, Tombstone→Compact)
//	otherwise          → current (no action)
//
// Returns current when nCtx is unavailable (≤0) so the caller treats
// compaction as disabled. Pure function — safe to call on every turn.
func CheckThreshold(promptTokens, responseReserveTokens, nCtx int, current Phase) Phase {
	if nCtx <= 0 {
		return current
	}
	used := float64(promptTokens + responseReserveTokens)
	budget := float64(nCtx)
	if used > thresholdPanic*budget {
		return PhasePanic
	}
	if used > thresholdEnterPhase2*budget {
		switch current {
		case PhaseRaw:
			return PhaseTombstone
		case PhaseTombstone:
			return PhaseCompact
		default:
			return current
		}
	}
	return current
}

// SessionEvent is one line from the JSONL session log.
type SessionEvent struct {
	UUID    string         `json:"uuid,omitempty"`
	Type    string         `json:"type,omitempty"`
	Message map[string]any `json:"message,omitempty"`
	Title   string         `json:"title,omitempty"`

	// Compaction-record fields (set on tombstone.applied / compaction.applied).
	EventID          string `json:"event_id,omitempty"`
	Strategy         string `json:"strategy,omitempty"`
	CompactedContent string `json:"compacted_content,omitempty"`
	OriginalSize     int    `json:"original_size,omitempty"`
}

// HydrationStrategy controls which compactions are eligible during hydration.
type HydrationStrategy struct {
	// PristineOnly = true ignores all compaction records (forensic raw replay).
	PristineOnly bool
	// AllowedStrategies is a set of compaction strategies eligible for use.
	// When nil/empty, all strategies are allowed and the most recent wins.
	AllowedStrategies map[string]bool
	// KTurns sets the size of the pristine tail (last K assistant turns
	// preserved verbatim). When 0, defaults to defaultPristineTailTurns.
	KTurns int
}

func (hs HydrationStrategy) accepts(strategy string) bool {
	if hs.PristineOnly {
		return false
	}
	if len(hs.AllowedStrategies) == 0 {
		return true
	}
	return hs.AllowedStrategies[strategy]
}

func (hs HydrationStrategy) kTurns() int {
	if hs.KTurns > 0 {
		return hs.KTurns
	}
	return defaultPristineTailTurns
}

// readSessionEvents parses every line of the JSONL session log at path.
// Lines that fail to parse are skipped silently — the log is best-effort and
// hydration must not fail because of one malformed entry.
func readSessionEvents(path string) ([]SessionEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []SessionEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024) // tool_results can be large
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev SessionEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		return out, err
	}
	return out, nil
}

// pristineBoundary returns the index of the first event in the pristine tail.
// The tail spans the last kTurns assistant events plus everything after them.
// When the log has fewer than kTurns assistant events, returns 0 (whole log
// is pristine).
func pristineBoundary(events []SessionEvent, kTurns int) int {
	if kTurns <= 0 {
		kTurns = defaultPristineTailTurns
	}
	seen := 0
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == "assistant" {
			seen++
			if seen == kTurns {
				return i
			}
		}
	}
	return 0
}

// pickCompaction returns the most recent compaction record for eventID whose
// strategy is accepted by hs, or nil if none.
func pickCompaction(events []SessionEvent, eventID string, hs HydrationStrategy) *SessionEvent {
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if ev.Type != "tombstone.applied" && ev.Type != "compaction.applied" {
			continue
		}
		if ev.EventID != eventID {
			continue
		}
		if !hs.accepts(ev.Strategy) {
			continue
		}
		return &events[i]
	}
	return nil
}

// findSummaryRecord returns the most recent compaction.applied with
// strategy=compaction-summary (the synthesized summary message), or nil.
func findSummaryRecord(events []SessionEvent, hs HydrationStrategy) *SessionEvent {
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if ev.Type != "compaction.applied" {
			continue
		}
		if ev.Strategy != StrategyCompactionSummary {
			continue
		}
		if !hs.accepts(ev.Strategy) {
			continue
		}
		return &events[i]
	}
	return nil
}

// hydrate reconstructs the chat msgs[] for a session from the JSONL log,
// applying compactions per the given strategy. system and originalUser are
// the system prompt + the original user task — both are emitted verbatim at
// the top and never compacted.
//
// The hydration order:
//
//  1. system prompt (if non-empty)
//  2. original user task
//  3. summary message (when a compaction-summary record exists)
//  4. each event in order, except:
//     - events whose UUID matches a rolled-into-summary record are skipped
//     - events outside the pristine tail get their most-recent matching
//     compaction substituted in for their content
//     - events inside the pristine tail are emitted verbatim
func hydrate(events []SessionEvent, system, originalUser string, hs HydrationStrategy) []llm.ChatMsg {
	var msgs []llm.ChatMsg
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, llm.ChatMsg{Role: "system", Content: system})
	}
	if strings.TrimSpace(originalUser) != "" {
		msgs = append(msgs, llm.ChatMsg{Role: "user", Content: originalUser})
	}

	if summary := findSummaryRecord(events, hs); summary != nil {
		msgs = append(msgs, llm.ChatMsg{Role: "assistant", Content: summary.CompactedContent})
	}

	tail := pristineBoundary(events, hs.kTurns())

	// Map tool_use_id -> tool name, accumulated as we walk assistant events.
	// Used to fill in ChatMsg.Name on tool messages (the JSONL tool_result
	// only carries tool_use_id).
	toolNames := map[string]string{}

	// Build a set of event UUIDs subsumed by the summary so they're skipped.
	rolled := map[string]bool{}
	for _, ev := range events {
		if ev.Type != "compaction.applied" {
			continue
		}
		if ev.Strategy != StrategyRolledIntoSummary {
			continue
		}
		if !hs.accepts(ev.Strategy) {
			continue
		}
		if ev.EventID != "" {
			rolled[ev.EventID] = true
		}
	}

	// The original user task in the JSONL appears as the very first user-text
	// event. We've already emitted originalUser; skip the matching event.
	originalUserEmitted := strings.TrimSpace(originalUser) != ""

	for i, ev := range events {
		// Compaction records and ai-title don't render as chat messages.
		if ev.Type == "tombstone.applied" || ev.Type == "compaction.applied" || ev.Type == "ai-title" {
			continue
		}
		if rolled[ev.UUID] {
			continue
		}
		if originalUserEmitted && isUserText(ev) {
			originalUserEmitted = false
			continue
		}

		isPristine := i >= tail
		var compaction *SessionEvent
		if !isPristine {
			compaction = pickCompaction(events, ev.UUID, hs)
		}

		switch ev.Type {
		case "user":
			msg := userEventToChatMsg(ev, compaction, toolNames)
			if msg != nil {
				msgs = append(msgs, *msg)
			}
		case "assistant":
			msg := assistantEventToChatMsg(ev, compaction, toolNames)
			if msg != nil {
				msgs = append(msgs, *msg)
			}
		}
	}
	return msgs
}

func isUserText(ev SessionEvent) bool {
	if ev.Type != "user" || ev.Message == nil {
		return false
	}
	if _, ok := ev.Message["content"].(string); ok {
		return true
	}
	return false
}

// userEventToChatMsg renders a user-typed event as either a user text message
// or a tool message. compaction (when non-nil) replaces the rendered content.
func userEventToChatMsg(ev SessionEvent, compaction *SessionEvent, toolNames map[string]string) *llm.ChatMsg {
	if ev.Message == nil {
		return nil
	}
	if text, ok := ev.Message["content"].(string); ok {
		// Plain user-text message.
		out := text
		if compaction != nil {
			out = compaction.CompactedContent
		}
		return &llm.ChatMsg{Role: "user", Content: out}
	}
	// Tool-result envelope: content is an array of {tool_use_id, type, content, is_error}.
	arr, _ := ev.Message["content"].([]any)
	if len(arr) == 0 {
		return nil
	}
	first, _ := arr[0].(map[string]any)
	if first == nil {
		return nil
	}
	if t, _ := first["type"].(string); t != "tool_result" {
		return nil
	}
	toolID, _ := first["tool_use_id"].(string)
	content, _ := first["content"].(string)
	if compaction != nil {
		content = compaction.CompactedContent
	}
	return &llm.ChatMsg{
		Role:       "tool",
		ToolCallID: toolID,
		Name:       toolNames[toolID],
		Content:    content,
	}
}

// assistantEventToChatMsg renders an assistant event back into a ChatMsg.
// compaction (when non-nil) replaces the text portion; tool_calls are
// preserved either way (the model needs them to stay paired with the
// downstream tool messages).
func assistantEventToChatMsg(ev SessionEvent, compaction *SessionEvent, toolNames map[string]string) *llm.ChatMsg {
	if ev.Message == nil {
		return nil
	}
	contentArr, _ := ev.Message["content"].([]any)
	if len(contentArr) == 0 {
		// Older logs may carry plain string content.
		if text, ok := ev.Message["content"].(string); ok {
			out := text
			if compaction != nil {
				out = compaction.CompactedContent
			}
			return &llm.ChatMsg{Role: "assistant", Content: out}
		}
		return nil
	}
	var text string
	var toolCalls []llm.ToolCall
	for _, raw := range contentArr {
		m, _ := raw.(map[string]any)
		if m == nil {
			continue
		}
		switch m["type"] {
		case "text":
			if s, ok := m["text"].(string); ok {
				text = s
			}
		case "tool_use":
			id, _ := m["id"].(string)
			name, _ := m["name"].(string)
			input := m["input"]
			argBytes, _ := json.Marshal(input)
			tc := llm.ToolCall{
				ID:   id,
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      name,
					Arguments: string(argBytes),
				},
			}
			toolCalls = append(toolCalls, tc)
			if id != "" && name != "" {
				toolNames[id] = name
			}
		}
	}
	if compaction != nil {
		text = compaction.CompactedContent
	}
	return &llm.ChatMsg{
		Role:      "assistant",
		Content:   text,
		ToolCalls: toolCalls,
	}
}

// tombstone10pct truncates content to its first ~10% of bytes (with a hard
// floor) and wraps the result in a structural marker the agent can recognize.
// The marker tells the agent it can rerun the originating tool to recover
// the full payload.
func tombstone10pct(orig string) string {
	if len(orig) < tombstoneMinBytes {
		return orig
	}
	cut := int(float64(len(orig)) * tombstoneFirstFraction)
	if cut < 256 {
		cut = 256
	}
	if cut > len(orig) {
		cut = len(orig)
	}
	prefix := orig[:cut]
	wrapper := map[string]any{
		"_tombstoned":      true,
		"_original_size":   len(orig),
		"_partial_content": prefix,
		"_hint":            "rerun the tool to recover full output",
	}
	buf, err := json.Marshal(wrapper)
	if err != nil {
		// Defensive fallback — should not happen for these primitive types.
		return orig[:cut]
	}
	return string(buf)
}

// tombstoneJSONWalk parses content as JSON and rewrites it into a marker that
// preserves all map keys and array shapes but truncates leaf string values to
// a small cap. Useful for tool_results that ARE JSON (e.g. structured API
// responses) — keeps enough shape signal that the agent can recognize what
// the result was without forcing it to rerun the tool.
//
// Falls back to tombstone10pct when content isn't valid JSON or is below the
// minimum size threshold.
func tombstoneJSONWalk(orig string) string {
	if len(orig) < tombstoneMinBytes {
		return orig
	}
	var v any
	if err := json.Unmarshal([]byte(orig), &v); err != nil {
		return tombstone10pct(orig)
	}
	walked := walkAndTruncate(v, 0, jsonWalkMaxDepth, jsonWalkLeafCap)
	wrapper := map[string]any{
		"_tombstoned":    true,
		"_strategy":      StrategyTombstoneJSONWalk,
		"_original_size": len(orig),
		"_walked":        walked,
		"_hint":          "rerun the tool to recover full output; keys/shape preserved",
	}
	buf, err := json.Marshal(wrapper)
	if err != nil {
		return tombstone10pct(orig)
	}
	return string(buf)
}

const (
	jsonWalkMaxDepth = 4
	jsonWalkLeafCap  = 64
)

// walkAndTruncate copies v, truncating any leaf string longer than leafCap
// and replacing values past maxDepth with a depth-elided placeholder.
func walkAndTruncate(v any, depth, maxDepth, leafCap int) any {
	if depth > maxDepth {
		return "…"
	}
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = walkAndTruncate(val, depth+1, maxDepth, leafCap)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = walkAndTruncate(val, depth+1, maxDepth, leafCap)
		}
		return out
	case string:
		if len(t) <= leafCap {
			return t
		}
		return t[:leafCap] + "…"
	default:
		return v
	}
}

// applyTombstone walks the event log and produces tombstone.applied records
// for every tool_result outside the pristine tail whose content is large
// and not already tombstoned. The records reference the original event UUID
// and carry the compacted content; callers append them to the JSONL log.
func applyTombstone(events []SessionEvent, hs HydrationStrategy) []SessionEvent {
	tail := pristineBoundary(events, hs.kTurns())
	var out []SessionEvent
	for i := 0; i < tail; i++ {
		ev := events[i]
		if ev.Type != "user" || ev.Message == nil {
			continue
		}
		// Already has a tombstone? Skip.
		if pickCompaction(events, ev.UUID, HydrationStrategy{}) != nil {
			continue
		}
		arr, _ := ev.Message["content"].([]any)
		if len(arr) == 0 {
			continue
		}
		first, _ := arr[0].(map[string]any)
		if first == nil {
			continue
		}
		if t, _ := first["type"].(string); t != "tool_result" {
			continue
		}
		content, _ := first["content"].(string)
		if len(content) < tombstoneMinBytes {
			continue
		}
		out = append(out, SessionEvent{
			Type:             "tombstone.applied",
			EventID:          ev.UUID,
			Strategy:         StrategyTombstone10pct,
			CompactedContent: tombstone10pct(content),
			OriginalSize:     len(content),
		})
	}
	return out
}

// applyCompaction synthesizes a four-section summary of all events outside
// the pristine tail, marks each subsumed event with a rolled-into-summary
// record, and emits one compaction.applied summary record. The summary is
// produced via a separate ChatCompletion call to summarizer (no tools, low
// max_tokens, focused prompt).
func applyCompaction(ctx context.Context, summarizer *llm.Client, events []SessionEvent, hs HydrationStrategy) ([]SessionEvent, error) {
	tail := pristineBoundary(events, hs.kTurns())
	if tail == 0 {
		return nil, nil
	}
	var transcript strings.Builder
	var rolledIDs []string
	for i := 0; i < tail; i++ {
		ev := events[i]
		if ev.Type != "user" && ev.Type != "assistant" {
			continue
		}
		rolledIDs = append(rolledIDs, ev.UUID)
		summarizeEventInto(&transcript, ev)
	}
	if transcript.Len() == 0 {
		return nil, nil
	}
	prompt := compactionSummaryPrompt() + "\n\n--- conversation ---\n" + transcript.String()
	summary, err := summarizer.Complete(ctx, []llm.ChatMessage{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, fmt.Errorf("compaction summary call: %w", err)
	}
	if len(summary) > compactionSummaryCharCap {
		summary = summary[:compactionSummaryCharCap]
	}

	out := make([]SessionEvent, 0, len(rolledIDs)+1)
	out = append(out, SessionEvent{
		Type:             "compaction.applied",
		Strategy:         StrategyCompactionSummary,
		CompactedContent: summary,
	})
	for _, id := range rolledIDs {
		out = append(out, SessionEvent{
			Type:     "compaction.applied",
			EventID:  id,
			Strategy: StrategyRolledIntoSummary,
		})
	}
	return out, nil
}

// summarizeEventInto writes a compact textual rendering of ev into b for
// inclusion in the compaction-summary prompt. We intentionally render
// truncated forms here — the summarizer doesn't need full tool_result
// payloads, just enough signal to identify what happened.
func summarizeEventInto(b *strings.Builder, ev SessionEvent) {
	if ev.Message == nil {
		return
	}
	switch ev.Type {
	case "user":
		if text, ok := ev.Message["content"].(string); ok {
			b.WriteString("USER: ")
			b.WriteString(truncForSummary(text, 1024))
			b.WriteString("\n")
			return
		}
		arr, _ := ev.Message["content"].([]any)
		for _, raw := range arr {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			if t, _ := m["type"].(string); t == "tool_result" {
				content, _ := m["content"].(string)
				b.WriteString("TOOL_RESULT: ")
				b.WriteString(truncForSummary(content, 512))
				b.WriteString("\n")
			}
		}
	case "assistant":
		arr, _ := ev.Message["content"].([]any)
		for _, raw := range arr {
			m, _ := raw.(map[string]any)
			if m == nil {
				continue
			}
			switch m["type"] {
			case "text":
				if s, ok := m["text"].(string); ok && strings.TrimSpace(s) != "" {
					b.WriteString("ASSISTANT: ")
					b.WriteString(truncForSummary(s, 1024))
					b.WriteString("\n")
				}
			case "tool_use":
				name, _ := m["name"].(string)
				args, _ := json.Marshal(m["input"])
				b.WriteString("TOOL_CALL: ")
				b.WriteString(name)
				b.WriteString(" ")
				b.WriteString(truncForSummary(string(args), 256))
				b.WriteString("\n")
			}
		}
	}
}

func truncForSummary(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func compactionSummaryPrompt() string {
	return strings.TrimSpace(`
Read the conversation below. Produce a structured handoff for your future
self with a fresh context window. Use these four labeled sections:

CONTEXT:
  Files touched (with paths), decisions made, what was verified vs
  attempted. Include specific identifiers. Third-person past tense.

STRATEGY:
  The plan you've been following. What approach? What patterns are you
  reusing? What tradeoffs did you commit to that constrain future choices?

NEXT STEPS:
  Concrete pending steps in order, each starting with a verb. Cap at 5.

CURRENT ACTION:
  Exactly what you were doing when context ran out. Be specific —
  'I was about to verify migration 153 by running ./bin/db migrate'
  not 'I was verifying things'.

The last tool call and its result will be appended verbatim after your
summary, so you don't need to restate them.
`)
}
