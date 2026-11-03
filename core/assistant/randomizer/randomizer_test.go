package randomizer

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name    string
		calls   []Call
		want    string
		wantErr bool
	}{
		{
			name:  "random_text single option",
			calls: []Call{{Fn: FnRandomText, Args: Args{N: 1, Options: []string{"only_one"}}}},
			want:  "only_one",
		},
		{
			name:  "random_num fixed range",
			calls: []Call{{Fn: FnRandomNum, Args: Args{N: 1, Min: 10, Max: 10}}},
			want:  "10",
		},
		{
			name: "multiple calls",
			calls: []Call{
				{Fn: FnRandomText, Args: Args{N: 1, Options: []string{"first"}}},
				{Fn: FnRandomNum, Args: Args{N: 1, Min: 2, Max: 2}},
			},
			want: "first\n2",
		},
		{
			name:  "repeatable forced when n exceeds options",
			calls: []Call{{Fn: FnRandomText, Args: Args{N: 3, Options: []string{"a"}}}},
			want:  "a a a",
		},
		{
			name:    "unsupported fn",
			calls:   []Call{{Fn: "os_exec"}},
			wantErr: true,
		},
		{
			name:    "empty options",
			calls:   []Call{{Fn: FnRandomText, Args: Args{N: 1}}},
			wantErr: true,
		},
		{
			name:    "min greater than max",
			calls:   []Call{{Fn: FnRandomNum, Args: Args{N: 1, Min: 5, Max: 1}}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Execute(tt.calls)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Execute() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecuteEmpty(t *testing.T) {
	if _, err := Execute(nil); !errors.Is(err, ErrNoResult) {
		t.Fatalf("Execute(nil) error = %v, want ErrNoResult", err)
	}
}

func TestRandomTextUnique(t *testing.T) {
	options := []string{"a", "b", "c", "d", "e"}
	for range 100 {
		got, err := randomText(Args{N: len(options), Options: options})
		if err != nil {
			t.Fatal(err)
		}
		sorted := slices.Sorted(slices.Values(got))
		if !slices.Equal(sorted, options) {
			t.Fatalf("randomText() = %v, want a permutation of %v", got, options)
		}
	}
}

func TestRandomNumUniqueWithinRange(t *testing.T) {
	for range 100 {
		got, err := randomNum(Args{N: 50, Min: -20, Max: 29})
		if err != nil {
			t.Fatal(err)
		}
		seen := make(map[int]bool, len(got))
		for _, s := range got {
			v, err := strconv.Atoi(s)
			if err != nil {
				t.Fatal(err)
			}
			if v < -20 || v > 29 || seen[v] {
				t.Fatalf("randomNum() = %v, contains out-of-range or duplicate %d", got, v)
			}
			seen[v] = true
		}
	}
}

func TestRandomNumHugeRange(t *testing.T) {
	got, err := randomNum(Args{N: 50, Min: -2_000_000_000, Max: 2_000_000_000})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 50 {
		t.Fatalf("len(randomNum()) = %d, want 50", len(got))
	}
}

func TestClampPickCount(t *testing.T) {
	for n, want := range map[int]int{-3: 1, 0: 1, 7: 7, 50: 50, 999: 50} {
		if got := clampPickCount(n); got != want {
			t.Errorf("clampPickCount(%d) = %d, want %d", n, got, want)
		}
	}
	if got := strings.Count(mustExecute(t, Call{Fn: FnRandomNum, Args: Args{N: 999, Min: 1, Max: 1}}), "1"); got != maxPickCount {
		t.Errorf("clamped output has %d items, want %d", got, maxPickCount)
	}
}

func mustExecute(t *testing.T, calls ...Call) string {
	t.Helper()
	got, err := Execute(calls)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
