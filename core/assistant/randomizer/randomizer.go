package randomizer

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

const (
	FnRandomText = "random_text"
	FnRandomNum  = "random_num"

	minPickCount = 1
	maxPickCount = 50
)

var ErrNoResult = errors.New("no result was generated")

type Call struct {
	Fn   string `json:"fn"`
	Args Args   `json:"args"`
}

type Args struct {
	IsRepeatable bool     `json:"is_repeatable"`
	N            int      `json:"n"`
	Options      []string `json:"options,omitempty"`
	Min          int      `json:"min,omitempty"`
	Max          int      `json:"max,omitempty"`
}

type handler func(args Args) ([]string, error)

var handlers = map[string]handler{
	FnRandomText: randomText,
	FnRandomNum:  randomNum,
}

func FnNames() []string {
	names := make([]string, 0, len(handlers))
	for name := range handlers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func Execute(calls []Call) (string, error) {
	lines := make([]string, 0, len(calls))
	for i, call := range calls {
		handle, ok := handlers[call.Fn]
		if !ok {
			return "", fmt.Errorf("call %d: unsupported fn %q", i, call.Fn)
		}
		items, err := handle(call.Args)
		if err != nil {
			return "", fmt.Errorf("call %d (%s): %w", i, call.Fn, err)
		}
		lines = append(lines, strings.Join(items, " "))
	}

	result := strings.Join(lines, "\n")
	if strings.TrimSpace(result) == "" {
		return "", ErrNoResult
	}
	return result, nil
}

func clampPickCount(n int) int {
	return min(max(n, minPickCount), maxPickCount)
}

func randomText(args Args) ([]string, error) {
	options := args.Options
	if len(options) == 0 {
		return nil, errors.New("options must not be empty")
	}

	n := clampPickCount(args.N)
	if args.IsRepeatable || n > len(options) {
		result := make([]string, n)
		for i := range result {
			result[i] = options[rand.IntN(len(options))]
		}
		return result, nil
	}

	shuffled := slices.Clone(options)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	return shuffled[:n], nil
}

func randomNum(args Args) ([]string, error) {
	if args.Min > args.Max {
		return nil, fmt.Errorf("min %d is greater than max %d", args.Min, args.Max)
	}

	span := int64(args.Max) - int64(args.Min) + 1
	n := clampPickCount(args.N)
	result := make([]string, n)

	if args.IsRepeatable || int64(n) > span {
		for i := range result {
			result[i] = strconv.FormatInt(int64(args.Min)+rand.Int64N(span), 10)
		}
		return result, nil
	}

	swapped := make(map[int64]int64, n*2)
	slotValue := func(slot int64) int64 {
		if v, ok := swapped[slot]; ok {
			return v
		}
		return slot
	}
	for i := range result {
		slot := int64(i)
		pick := slot + rand.Int64N(span-slot)
		picked := slotValue(pick)
		swapped[pick] = slotValue(slot)
		result[i] = strconv.FormatInt(int64(args.Min)+picked, 10)
	}
	return result, nil
}
