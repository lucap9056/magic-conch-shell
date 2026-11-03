package discordclient

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/v3/structs"
	"github.com/lucap9056/magic-conch-shell/core/v3/structs/languages"

	COMMANDS "github.com/lucap9056/magic-conch-shell/discord/v2/internal/commands"
	OPTIONS "github.com/lucap9056/magic-conch-shell/discord/v2/internal/options"

	"github.com/bwmarrin/discordgo"
)

const (
	maxContextDepth = 20
	silentReply     = "The shell remains silent."
)

type GenerateResponse = func(context.Context, languages.Language, *structs.PromptMessage, []*structs.PromptMessage) (string, error)

type DiscordClient struct {
	session          *discordgo.Session
	channels         *GuildResponseChannels
	generateResponse GenerateResponse
	language         languages.Language
}

func New(token string, language string, generateResponse GenerateResponse) (*DiscordClient, error) {

	channels, err := newGuildResponseChannels()
	if err != nil {
		return nil, fmt.Errorf("failed to create guild response channels: %w", err)
	}

	token = strings.TrimPrefix(token, "Bot ")

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	lang, _ := languages.Parse(language)

	client := &DiscordClient{session, channels, generateResponse, lang}

	session.AddHandler(client.interactionCreateHandler)
	session.AddHandler(client.messageCreateHandler)

	return client, nil
}

func (client *DiscordClient) Open() error {
	err := client.session.Open()
	if err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}
	log.Println("Discord session opened successfully.")
	return nil
}

func (client *DiscordClient) Close() {
	client.session.Close()
	client.channels.Close()
}

func (client *DiscordClient) interactionCreateHandler(session *discordgo.Session, interaction *discordgo.InteractionCreate) {

	command, err := newSlashCommand(session, interaction)
	if err != nil {
		return
	}

	switch command.GetName() {
	case COMMANDS.CHANNEL:
		member := interaction.Member
		if member == nil {
			return
		}

		isAdmin := member.Permissions&discordgo.PermissionAdministrator != 0

		if !isAdmin {
			return
		}

		channel := command.GetChannelOption(OPTIONS.CHANNEL)
		if channel == nil {
			return
		}

		guildID := interaction.GuildID
		channelID := channel.ID

		err := client.channels.SetResponseChannel(guildID, channelID)
		reply := "Success"
		if err != nil {
			reply = "Failed"
			log.Println(err)
		}

		session.InteractionRespond(
			interaction.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: reply,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			},
		)

	case COMMANDS.QUATION:
		message := command.GetStringOption(OPTIONS.TEXT)
		if message == "" {
			return
		}

		session.InteractionRespond(
			interaction.Interaction,
			&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
				},
			},
		)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		req := &structs.PromptMessage{
			Parts: []*structs.PromptPart{
				structs.NewTextPart(message),
			},
		}

		lang, _ := languages.Parse(string(interaction.Locale))

		reply, err := client.generateResponse(ctx, lang, req, nil)

		if err != nil {

			log.Println("=====")
			log.Println("Error occurred while processing the interaction.")
			log.Println(message)
			log.Println(err)

		}

		session.FollowupMessageCreate(
			interaction.Interaction,
			true,
			&discordgo.WebhookParams{
				Content: replyOrFallback(reply),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		)

	default:
		return
	}

}

func (client *DiscordClient) messageCreateHandler(session *discordgo.Session, message *discordgo.MessageCreate) {

	if message.Author == nil || message.Author.Bot {
		return
	}

	guildID := message.GuildID
	targetChannelID, found := client.channels.GetResponseChannel(guildID)
	content, isMentioned := stripBotMention(message.Content, client.session.State.User.ID)

	if (!found || message.ChannelID != targetChannelID) && !isMentioned {
		return
	}

	currentUserMessage := buildPromptMessage(message.Attachments, content)
	if len(currentUserMessage.Parts) == 0 {
		return
	}

	// Fetch referenced messages (context)
	referencedMessages, err := client.fetchMessageContext(message.MessageReference, 0)
	if err != nil {
		log.Printf("Error fetching referenced messages for message %s: %v", message.ID, err)

		session.ChannelMessageSendReply(
			message.ChannelID,
			"Failed to fetch message context.",
			&discordgo.MessageReference{
				MessageID: message.ID,
			},
		)
		return
	}

	// Convert Discord messages to LLM prompt messages
	promptMessages := make([]*structs.PromptMessage, len(referencedMessages))
	for i, msg := range referencedMessages {
		promptMessages[i] = client.convertDiscordMessageToPrompt(msg)
	}

	// Prepare context for API call
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	response, err := client.generateResponse(ctx, client.language, currentUserMessage, promptMessages)
	if err != nil {
		log.Printf("Failed to generate response for message %s: %v", message.ID, err)
		log.Printf("Message content: %s", content)

	}

	session.ChannelMessageSendReply(
		message.ChannelID,
		replyOrFallback(response),
		&discordgo.MessageReference{
			MessageID: message.ID,
		},
	)
}
func (dc *DiscordClient) convertDiscordMessageToPrompt(m *discordgo.Message) *structs.PromptMessage {
	return buildPromptMessage(m.Attachments, m.Content)
}

func buildPromptMessage(attachments []*discordgo.MessageAttachment, text string) *structs.PromptMessage {
	parts := []*structs.PromptPart{}

	// Handle attachments
	for _, attachment := range attachments {
		if strings.HasPrefix(attachment.ContentType, "image/") {
			parts = append(parts, structs.NewImagePart(attachment.ContentType, attachment.URL))
		}
	}

	// Add text content
	if text != "" {
		parts = append(parts, structs.NewTextPart(text))
	}

	return &structs.PromptMessage{
		Parts: parts,
	}
}

func stripBotMention(content string, botID string) (string, bool) {
	for _, mention := range []string{"<@" + botID + ">", "<@!" + botID + ">"} {
		if rest, ok := strings.CutPrefix(content, mention); ok {
			return strings.TrimSpace(rest), true
		}
	}
	return content, false
}

func replyOrFallback(reply string) string {
	if strings.TrimSpace(reply) == "" {
		return silentReply
	}
	return reply
}

// fetchMessageContext recursively fetches the full chain of referenced messages to provide context.
func (dc *DiscordClient) fetchMessageContext(reference *discordgo.MessageReference, depth int) ([]*discordgo.Message, error) {
	if reference == nil || reference.MessageID == "" || depth >= maxContextDepth {
		return nil, nil
	}

	// Fetch the referenced message
	msg, err := dc.session.ChannelMessage(reference.ChannelID, reference.MessageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch referenced message %s: %w", reference.MessageID, err)
	}

	// If this message references another, recurse
	if msg.MessageReference != nil && msg.MessageReference.MessageID != "" {
		previousMessages, err := dc.fetchMessageContext(msg.MessageReference, depth+1)
		if err != nil {
			return nil, err
		}
		return append(previousMessages, msg), nil
	}

	return []*discordgo.Message{msg}, nil
}
