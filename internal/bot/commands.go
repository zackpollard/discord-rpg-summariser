package bot

import "github.com/bwmarrin/discordgo"

// commands defines the slash command tree registered with Discord.
var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "session",
		Description: "Manage RPG recording sessions",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "start",
				Description: "Start recording in your current voice channel",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "stop",
				Description: "Stop recording and begin processing",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "status",
				Description: "Show current session info",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
		},
	},
	{
		Name:        "character",
		Description: "Manage character name mappings",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "set",
				Description: "Set your character name (or another user's)",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "name",
						Description: "Character name",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
					{
						Name:        "user",
						Description: "User to set the name for (defaults to you)",
						Type:        discordgo.ApplicationCommandOptionUser,
						Required:    false,
					},
				},
			},
			{
				Name:        "list",
				Description: "List all character mappings for this server",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "remove",
				Description: "Remove a character mapping",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "user",
						Description: "User to remove (defaults to you)",
						Type:        discordgo.ApplicationCommandOptionUser,
						Required:    false,
					},
				},
			},
		},
	},
}
