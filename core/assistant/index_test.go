package assistant

import (
	"testing"
)

func TestSafeExecuteLua(t *testing.T) {
	c := &Client{}

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
