package assistant

import (
	"testing"

	"github.com/lucap9056/magic-conch-shell/core/v3/assistant/luascript"
)

func TestSafeExecuteLua(t *testing.T) {
	c := &Client{luaRuntimeScript: luascript.GetScript()}

	tests := []struct {
		name    string
		luaCode string
		want    string
		wantErr bool
	}{
		{
			name:    "random_text single option",
			luaCode: `random_text(false, 1, "only_one")`,
			want:    "only_one",
		},
		{
			name:    "random_num range",
			luaCode: `random_num(false, 1, 10, 10)`,
			want:    "10",
		},
		{
			name:    "multiple calls",
			luaCode: `random_text(false, 1, "first"); random_num(false, 1, 2, 2)`,
			want:    "first\n2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.safeExecuteLua(tt.luaCode)
			if (err != nil) != tt.wantErr {
				t.Errorf("safeExecuteLua() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("safeExecuteLua() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecuteCalls(t *testing.T) {
	tests := []struct {
		name      string
		llmOutput string
		want      string
		wantErr   bool
	}{
		{
			name:      "single call",
			llmOutput: `[{"fn":"random_text","args":{"is_repeatable":false,"n":1,"options":["only_one"]}}]`,
			want:      "only_one",
		},
		{
			name:      "multiple calls",
			llmOutput: `[{"fn":"random_text","args":{"is_repeatable":false,"n":1,"options":["first"]}},{"fn":"random_num","args":{"is_repeatable":false,"n":1,"min":2,"max":2}}]`,
			want:      "first\n2",
		},
		{
			name:      "empty array means unknown",
			llmOutput: `[]`,
			want:      "I don't know how to choose from that.",
		},
		{
			name:      "malformed json",
			llmOutput: `random_text(false, 1, "a")`,
			want:      "Failed to process selection logic.",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executeCalls(tt.llmOutput)
			if (err != nil) != tt.wantErr {
				t.Fatalf("executeCalls() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("executeCalls() = %q, want %q", got, tt.want)
			}
		})
	}
}
