package depolycommands

import (
	"strings"

	COMMANDS "github.com/lucap9056/magic-conch-shell/discord/internal/commands"
	OPTIONS "github.com/lucap9056/magic-conch-shell/discord/internal/options"

	"github.com/bwmarrin/discordgo"
)

type Config struct {
	Token   string
	AppID   string
	GuildID string
}

func Depoly(config *Config) error {

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        COMMANDS.CHANNEL,
			Description: "set channel",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionChannel,
					Name:        OPTIONS.CHANNEL,
					Description: "channel",
					Required:    true,
				},
			},
		},
		{
			Name:        COMMANDS.QUATION,
			Description: "q",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        OPTIONS.TEXT,
					Description: "message",
					Required:    true,
				},
			},
		},
	}

	token := strings.TrimPrefix(config.Token, "Bot ")
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return err
	}

	for _, v := range commands {
		_, err := session.ApplicationCommandCreate(config.AppID, config.GuildID, v)
		if err != nil {
			return err
		}
	}

	return nil
}
