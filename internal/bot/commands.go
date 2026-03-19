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
	{
		Name:        "campaign",
		Description: "Manage RPG campaigns",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "create",
				Description: "Create a new campaign",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "name",
						Description: "Campaign name",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
					{
						Name:        "description",
						Description: "Campaign description",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    false,
					},
				},
			},
			{
				Name:        "list",
				Description: "List campaigns",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "set",
				Description: "Set the active campaign",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "name",
						Description: "Campaign name",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
				},
			},
			{
				Name:        "dm",
				Description: "Set the Dungeon Master for the active campaign",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "user",
						Description: "The DM (defaults to you)",
						Type:        discordgo.ApplicationCommandOptionUser,
						Required:    false,
					},
				},
			},
			{
				Name:        "recap",
				Description: "Generate or view the story so far",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "telegram-dm",
				Description: "Set the Telegram user ID of the DM for Telegram integration",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "telegram_user_id",
						Description: "The DM's Telegram numeric user ID",
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    true,
					},
				},
			},
		},
	},
	{
		Name:        "quest",
		Description: "Manage campaign quests",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "list",
				Description: "List all quests for the active campaign",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
			},
			{
				Name:        "complete",
				Description: "Mark a quest as completed",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "name",
						Description: "Quest name",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
				},
			},
			{
				Name:        "fail",
				Description: "Mark a quest as failed",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "name",
						Description: "Quest name",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
				},
			},
		},
	},
}
