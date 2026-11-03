package assistant

import (
	"testing"
)

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
