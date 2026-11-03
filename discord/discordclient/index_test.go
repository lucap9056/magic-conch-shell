package discordclient

import (
	"testing"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"

	"github.com/bwmarrin/discordgo"
)

func TestStripBotMention(t *testing.T) {
	const botID = "123"
	tests := []struct {
		name          string
		content       string
		wantContent   string
		wantMentioned bool
	}{
		{name: "user mention", content: "<@123> pizza or sushi?", wantContent: "pizza or sushi?", wantMentioned: true},
		{name: "nickname mention", content: "<@!123>   yes or no", wantContent: "yes or no", wantMentioned: true},
		{name: "content sharing mention characters is kept", content: "<@123> 3 <@ 12", wantContent: "3 <@ 12", wantMentioned: true},
		{name: "mention only", content: "<@123>", wantContent: "", wantMentioned: true},
		{name: "other user mention", content: "<@456> hello", wantContent: "<@456> hello", wantMentioned: false},
		{name: "mention not at start", content: "hey <@123>", wantContent: "hey <@123>", wantMentioned: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, mentioned := stripBotMention(tt.content, botID)
			if content != tt.wantContent || mentioned != tt.wantMentioned {
				t.Errorf("stripBotMention(%q) = (%q, %v), want (%q, %v)", tt.content, content, mentioned, tt.wantContent, tt.wantMentioned)
			}
		})
	}
}

func TestBuildPromptMessage(t *testing.T) {
	attachments := []*discordgo.MessageAttachment{
		{ContentType: "image/png", URL: "https://cdn.discordapp.com/a.png"},
		{ContentType: "application/pdf", URL: "https://cdn.discordapp.com/b.pdf"},
	}

	message := buildPromptMessage(attachments, "which one?")
	if len(message.Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(message.Parts))
	}
	if part := message.Parts[0]; part.Type != structs.PartTypeImage || part.Content != "https://cdn.discordapp.com/a.png" || part.MimeType != "image/png" {
		t.Errorf("image part = %+v", part)
	}
	if part := message.Parts[1]; part.Type != structs.PartTypeText || part.Content != "which one?" {
		t.Errorf("text part = %+v", part)
	}

	if empty := buildPromptMessage(nil, ""); len(empty.Parts) != 0 {
		t.Errorf("empty message parts = %d, want 0", len(empty.Parts))
	}
}

func TestReplyOrFallback(t *testing.T) {
	for reply, want := range map[string]string{"Yes": "Yes", "": silentReply, "  ": silentReply} {
		if got := replyOrFallback(reply); got != want {
			t.Errorf("replyOrFallback(%q) = %q, want %q", reply, got, want)
		}
	}
}

func TestInteractionLocaleParsing(t *testing.T) {
	tests := map[discordgo.Locale]languages.Language{
		discordgo.ChineseTW: languages.ZH,
		discordgo.ChineseCN: languages.ZH,
		discordgo.EnglishUS: languages.EN,
		discordgo.Japanese:  languages.EN,
	}
	for locale, want := range tests {
		if got, _ := languages.Parse(string(locale)); got != want {
			t.Errorf("languages.Parse(%q) = %q, want %q", locale, got, want)
		}
	}
}
