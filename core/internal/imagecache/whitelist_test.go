package imagecache

import "testing"

func TestParseDomainWhitelist(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		allowed  []string
		rejected []string
	}{
		{
			name:     "empty blocks everything",
			input:    "",
			rejected: []string{"", "cdn.discordapp.com"},
		},
		{
			name:     "only separators blocks everything",
			input:    " , ,",
			rejected: []string{"", "cdn.discordapp.com"},
		},
		{
			name:     "trims spaces and ignores case",
			input:    " cdn.discordapp.com , Media.DiscordApp.net ",
			allowed:  []string{"cdn.discordapp.com", "CDN.DiscordApp.com", "media.discordapp.net"},
			rejected: []string{"", "evil.com", "sub.cdn.discordapp.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whitelist := ParseDomainWhitelist(tt.input)
			for _, host := range tt.allowed {
				if !whitelist.IsAllowed(host) {
					t.Errorf("IsAllowed(%q) = false, want true", host)
				}
			}
			for _, host := range tt.rejected {
				if whitelist.IsAllowed(host) {
					t.Errorf("IsAllowed(%q) = true, want false", host)
				}
			}
		})
	}
}
