package assistant

import (
	"github.com/lucap9056/magic-conch-shell/core/v3/assistant/randomizer"

	"google.golang.org/genai"
)

var callsSchema = &genai.Schema{
	Type: genai.TypeArray,
	Items: &genai.Schema{
		Type:             genai.TypeObject,
		PropertyOrdering: []string{"fn", "args"},
		Required:         []string{"fn", "args"},
		Properties: map[string]*genai.Schema{
			"fn": {
				Type: genai.TypeString,
				Enum: randomizer.FnNames(),
			},
			"args": {
				Type:             genai.TypeObject,
				PropertyOrdering: []string{"is_repeatable", "n", "options", "min", "max"},
				Required:         []string{"is_repeatable", "n"},
				Properties: map[string]*genai.Schema{
					"is_repeatable": {Type: genai.TypeBoolean},
					"n":             {Type: genai.TypeInteger},
					"options": {
						Type:  genai.TypeArray,
						Items: &genai.Schema{Type: genai.TypeString},
					},
					"min": {Type: genai.TypeInteger},
					"max": {Type: genai.TypeInteger},
				},
			},
		},
	},
}
