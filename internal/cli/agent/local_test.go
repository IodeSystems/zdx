package agent

import (
	"testing"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func strp(s string) *string { return &s }

func TestPickComplexityModel(t *testing.T) {
	cfg := func(low, med, high string, xh, mx *string) dxclient.LLMConfigBody {
		return dxclient.LLMConfigBody{
			ModelLow:    low,
			ModelMedium: med,
			ModelHigh:   high,
			ModelXhigh:  xh,
			ModelMax:    mx,
		}
	}

	cases := []struct {
		name       string
		configs    []dxclient.LLMConfigBody
		complexity string
		want       string
		wantWarn   string // tier we expect warnFn to fire on, "" = no warn
	}{
		{
			name:       "xhigh resolves to dedicated slot when set",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "h", strp("xh"), strp("mx"))},
			complexity: ComplexityXHigh,
			want:       "xh",
		},
		{
			name:       "xhigh falls through to high with warn when nil",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "h", nil, nil)},
			complexity: ComplexityXHigh,
			want:       "h",
			wantWarn:   ComplexityXHigh,
		},
		{
			name:       "xhigh falls through to high with warn when empty-string-pointer",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "h", strp(""), strp(""))},
			complexity: ComplexityXHigh,
			want:       "h",
			wantWarn:   ComplexityXHigh,
		},
		{
			name:       "xhigh: empty xhigh+high in first row falls through to next row",
			configs:    []dxclient.LLMConfigBody{cfg("l1", "m1", "", nil, nil), cfg("l2", "m2", "h2", strp("xh2"), nil)},
			complexity: ComplexityXHigh,
			want:       "xh2",
		},
		{
			name:       "max resolves to dedicated slot when set",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "h", nil, strp("mx"))},
			complexity: ComplexityMax,
			want:       "mx",
		},
		{
			name:       "max falls through to high with warn when nil",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "h", nil, nil)},
			complexity: ComplexityMax,
			want:       "h",
			wantWarn:   ComplexityMax,
		},
		{
			name:       "low resolves direct slot, no warn",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "h", nil, nil)},
			complexity: ComplexityLow,
			want:       "l",
		},
		{
			name:       "high resolves direct slot, no warn",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "h", nil, nil)},
			complexity: ComplexityHigh,
			want:       "h",
		},
		{
			name:       "medium (default branch) resolves to model_medium",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "h", nil, nil)},
			complexity: ComplexityMedium,
			want:       "m",
		},
		{
			name:       "no configs returns empty",
			configs:    nil,
			complexity: ComplexityXHigh,
			want:       "",
		},
		{
			name:       "all rows empty for tier returns empty, no warn (nothing to fall through to)",
			configs:    []dxclient.LLMConfigBody{cfg("l", "m", "", nil, nil)},
			complexity: ComplexityXHigh,
			want:       "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var warned string
			warnFn := func(tier string) { warned = tier }
			got := pickComplexityModel(tc.configs, tc.complexity, warnFn)
			if got != tc.want {
				t.Errorf("pickComplexityModel: got %q want %q", got, tc.want)
			}
			if warned != tc.wantWarn {
				t.Errorf("warn: got %q want %q", warned, tc.wantWarn)
			}
		})
	}
}
