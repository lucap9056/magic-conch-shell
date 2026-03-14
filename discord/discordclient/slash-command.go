package discordclient

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type slashCommandOptions = map[string]*discordgo.ApplicationCommandInteractionDataOption

type slashCommand struct {
	session *discordgo.Session
	command discordgo.ApplicationCommandInteractionData
	options slashCommandOptions
}

func newSlashCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) (*slashCommand, error) {

	if interaction.Type != discordgo.InteractionApplicationCommand {
		return nil, fmt.Errorf("NewSlashCommand: invalid interaction type, expected ApplicationCommand but got %s", interaction.Type)
	}

	command := interaction.ApplicationCommandData()

	options := make(slashCommandOptions)

	for _, option := range command.Options {
		options[option.Name] = option
	}

	return &slashCommand{session, command, options}, nil
}

func (c *slashCommand) GetName() string {
	return c.command.Name
}

func (c *slashCommand) GetChannelOption(name string) *discordgo.Channel {
	option, ok := c.options[name]
	if ok && option.Type == discordgo.ApplicationCommandOptionChannel {
		channel := option.ChannelValue(c.session)
		if channel != nil {
			return channel
		}
	}
	return nil
}

func (c *slashCommand) GetStringOption(name string) string {
	option, ok := c.options[name]
	if ok && option.Type == discordgo.ApplicationCommandOptionString {
		return option.StringValue()
	}
	return ""
}
