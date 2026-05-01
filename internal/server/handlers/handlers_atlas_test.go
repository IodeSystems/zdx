package handlers

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestClampDepth(t *testing.T) {
	cases := []struct {
		in, want int32
	}{
		{-5, 0},
		{-1, 0},
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 2},
		{99, 2},
	}
	for _, tc := range cases {
		if got := clampDepth(tc.in); got != tc.want {
			t.Errorf("clampDepth(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestComputeStaleFraction(t *testing.T) {
	cases := []struct {
		name         string
		stale, total int64
		want         float64
	}{
		{"empty denom", 0, 0, 0.0},
		{"negative denom guarded", 5, -1, 0.0},
		{"all stale", 4, 4, 1.0},
		{"none stale", 0, 4, 0.0},
		{"half stale", 2, 4, 0.5},
		{"one of three", 1, 3, 1.0 / 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeStaleFraction(tc.stale, tc.total); got != tc.want {
				t.Errorf("computeStaleFraction(%d,%d) = %v, want %v",
					tc.stale, tc.total, got, tc.want)
			}
		})
	}
}

func TestSummarizeDescription(t *testing.T) {
	t.Run("short body untouched", func(t *testing.T) {
		got := summarizeDescription("hello world")
		if got != "hello world" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		got := summarizeDescription("  hi there  ")
		if got != "hi there" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("empty stays empty", func(t *testing.T) {
		if got := summarizeDescription(""); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("truncates past 200 runes with ellipsis", func(t *testing.T) {
		long := strings.Repeat("a", 250)
		got := summarizeDescription(long)
		// Ellipsis is "…" (one rune, three bytes) appended after truncation.
		if !strings.HasSuffix(got, "…") {
			t.Errorf("expected ellipsis suffix, got %q", got)
		}
		// 200 ASCII runes + 1 ellipsis rune.
		if utf8.RuneCountInString(got) != 201 {
			t.Errorf("expected 201 runes, got %d (%q)", utf8.RuneCountInString(got), got)
		}
	})

	t.Run("multibyte runes counted by codepoint", func(t *testing.T) {
		// 250 multibyte codepoints; should truncate at 200 runes.
		long := strings.Repeat("é", 250)
		got := summarizeDescription(long)
		if utf8.RuneCountInString(got) != 201 {
			t.Errorf("expected 201 runes, got %d", utf8.RuneCountInString(got))
		}
	})
}
