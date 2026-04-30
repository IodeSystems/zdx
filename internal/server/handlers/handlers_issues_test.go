package handlers

import (
	"reflect"
	"testing"
)

func TestExtractIssueRefs(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		excludeID string
		want      []string
	}{
		{
			name: "empty text",
			text: "",
			want: nil,
		},
		{
			name: "no refs",
			text: "Some plain text without any issue references.",
			want: nil,
		},
		{
			name: "single ref",
			text: "This unblocks IS-100.",
			want: []string{"IS-100"},
		},
		{
			name: "multiple refs in order",
			text: "Depends on IS-100 and IS-200; also blocks IS-50.",
			want: []string{"IS-100", "IS-200", "IS-50"},
		},
		{
			name: "dedup duplicate refs",
			text: "IS-100 is the parent. See IS-100 above. Also IS-200 IS-100.",
			want: []string{"IS-100", "IS-200"},
		},
		{
			name:      "exclude self ID",
			text:      "Self-reference IS-789 should be excluded; IS-100 should appear.",
			excludeID: "IS-789",
			want:      []string{"IS-100"},
		},
		{
			name: "word boundary excludes IS-N within longer tokens",
			text: "MIS-1 should not match. IS-12abc should not match either. But IS-12 alone should.",
			want: []string{"IS-12"},
		},
		{
			name: "cross-tracker step references",
			text: "Unblocks IS-610 step 6 (the fan-out). Companion to IS-648.",
			want: []string{"IS-610", "IS-648"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractIssueRefs(tc.text, tc.excludeID)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("refs mismatch\n  text: %q\n  exclude: %q\n  got:  %#v\n  want: %#v", tc.text, tc.excludeID, got, tc.want)
			}
		})
	}
}
