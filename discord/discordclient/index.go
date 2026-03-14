package discordclient

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lucap9056/magic-conch-shell/core/structs"

	COMMANDS "github.com/lucap9056/magic-conch-shell/discord/internal/commands"
	OPTIONS "github.com/lucap9056/magic-conch-shell/discord/internal/options"

	"github.com/bwmarrin/discordgo"
)

type GenerateResponse = func(context.Context, *structs.PromptMessage, []*structs.PromptMessage) (string, error)

type DiscordClient struct {
	session          *discordgo.Session
	channels         *GuildResponseChannels
	generateResponse GenerateResponse
}

func New(token string, generateResponse GenerateResponse) (*DiscordClient, error) {

	channels, err := newGuildResponseChannels()
	if err != nil {
		return nil, fmt.Errorf("failed to create guild response channels: %w", err)
	}

	token = strings.TrimPrefix(token, "Bot ")

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	client := &DiscordClient{session, channels, generateResponse}

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

		reply, err := client.generateResponse(ctx, req, nil)

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
				Content: reply,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		)

	default:
		return
	}

}

func (client *DiscordClient) messageCreateHandler(session *discordgo.Session, message *discordgo.MessageCreate) {

	if message.Author.Bot {
		return
	}

	guildID := message.GuildID
	targetChannelID, found := client.channels.GetResponseChannel(guildID)
	content := message.Content
	botMention := fmt.Sprintf("<@%s>", client.session.State.User.ID)
	isMentioned := strings.HasPrefix(content, botMention)

	if (!found || message.ChannelID != targetChannelID) && !isMentioned {
		return
	}

	if isMentioned {
		content = strings.TrimLeft(content, botMention)
		content = strings.TrimSpace(content)
	}

	// Fetch referenced messages (context)
	referencedMessages, err := client.fetchMessageContext(message.MessageReference)
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

	// Convert current message to LLM prompt message
	currentUserMessage := client.convertDiscordMessageToPrompt(message.Message)

	response, err := client.generateResponse(ctx, currentUserMessage, promptMessages)
	if err != nil {
		log.Printf("Failed to generate response for message %s: %v", message.ID, err)
		log.Printf("Message content: %s", content)

	}

	session.ChannelMessageSendReply(
		message.ChannelID,
		response,
		&discordgo.MessageReference{
			MessageID: message.ID,
		},
	)
}
func (dc *DiscordClient) convertDiscordMessageToPrompt(m *discordgo.Message) *structs.PromptMessage {
	parts := []*structs.PromptPart{}

	// Handle attachments
	for _, attachment := range m.Attachments {
		if strings.HasPrefix(attachment.ContentType, "image/") {
			parts = append(parts, structs.NewImagePart(attachment.ContentType, attachment.URL))
		}
	}

	// Add text content
	if m.Content != "" {
		parts = append(parts, structs.NewTextPart(m.Content))
	}

	return &structs.PromptMessage{
		Parts: parts,
	}
}

// fetchMessageContext recursively fetches the full chain of referenced messages to provide context.
func (dc *DiscordClient) fetchMessageContext(reference *discordgo.MessageReference) ([]*discordgo.Message, error) {
	if reference == nil || reference.MessageID == "" {
		return nil, nil
	}

	// Fetch the referenced message
	msg, err := dc.session.ChannelMessage(reference.ChannelID, reference.MessageID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch referenced message %s: %w", reference.MessageID, err)
	}

	// If this message references another, recurse
	if msg.MessageReference != nil && msg.MessageReference.MessageID != "" {
		previousMessages, err := dc.fetchMessageContext(msg.MessageReference)
		if err != nil {
			return nil, err
		}
		return append(previousMessages, msg), nil
	}

	return []*discordgo.Message{msg}, nil
}
