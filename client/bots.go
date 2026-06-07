package client

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// BotCommand represents a single bot command entry shown in the Telegram UI.
type BotCommand struct {
	Command     string
	Description string
}

// BotInfo holds the bot's display name, description and about text.
type BotInfo struct {
	Name        string
	Description string
	About       string
}

// AnswerInlineQueryParams holds all parameters for answering an inline query.
type AnswerInlineQueryParams struct {
	QueryID       int64
	Results       []tg.InputBotInlineResultClass
	CacheTime     int
	IsPersonal    bool
	NextOffset    string
	SwitchPM      string
	SwitchPMParam string
	Gallery       bool
	Private       bool
}

// AnswerInlineQuery sends results to an inline query.
func (c *MCUBClient) AnswerInlineQuery(ctx context.Context, params AnswerInlineQueryParams) error {
	req := &tg.MessagesSetInlineBotResultsRequest{
		QueryID:    params.QueryID,
		Results:    params.Results,
		CacheTime:  params.CacheTime,
		NextOffset: params.NextOffset,
	}
	if params.Gallery {
		req.SetGallery(true)
	}
	if params.Private {
		req.Private = true
	}
	if params.SwitchPM != "" {
		req.SetSwitchPm(tg.InlineBotSwitchPM{
			Text:       params.SwitchPM,
			StartParam: params.SwitchPMParam,
		})
	}
	_, err := c.api.MessagesSetInlineBotResults(ctx, req)
	if err != nil {
		return fmt.Errorf("answer inline query: %w", err)
	}
	return nil
}

// AnswerCallbackQuery answers a callback query triggered by an inline button press.
// alert causes the answer to be shown as an alert dialog rather than a notification.
// url, if non-empty, is a URL to open in the browser.
// cacheTime is how long (in seconds) the client may cache this answer.
func (c *MCUBClient) AnswerCallbackQuery(ctx context.Context, queryID int64, text string, alert bool, url string, cacheTime int) error {
	_, err := c.api.MessagesSetBotCallbackAnswer(ctx, &tg.MessagesSetBotCallbackAnswerRequest{
		QueryID:   queryID,
		Message:   text,
		Alert:     alert,
		URL:       url,
		CacheTime: cacheTime,
	})
	if err != nil {
		return fmt.Errorf("answer callback query: %w", err)
	}
	return nil
}

// SetBotCommands sets the bot command list shown in the Telegram UI.
// scope may be empty ("default"), "private", "groups", "group_admins", or "all_private_chats".
// langCode is the two-letter ISO 639-1 language code; empty string means the default.
func (c *MCUBClient) SetBotCommands(ctx context.Context, commands []BotCommand, scope string, langCode string) error {
	tlCommands := make([]tg.BotCommand, len(commands))
	for i, cmd := range commands {
		tlCommands[i] = tg.BotCommand{
			Command:     cmd.Command,
			Description: cmd.Description,
		}
	}
	botScope := botScopeFromString(scope)
	_, err := c.api.BotsSetBotCommands(ctx, &tg.BotsSetBotCommandsRequest{
		Scope:    botScope,
		LangCode: langCode,
		Commands: tlCommands,
	})
	if err != nil {
		return fmt.Errorf("set bot commands: %w", err)
	}
	return nil
}

// GetBotCommands returns the current bot command list for the given scope and language.
func (c *MCUBClient) GetBotCommands(ctx context.Context, scope string, langCode string) ([]BotCommand, error) {
	result, err := c.api.BotsGetBotCommands(ctx, &tg.BotsGetBotCommandsRequest{
		Scope:    botScopeFromString(scope),
		LangCode: langCode,
	})
	if err != nil {
		return nil, fmt.Errorf("get bot commands: %w", err)
	}
	out := make([]BotCommand, len(result))
	for i, cmd := range result {
		out[i] = BotCommand{
			Command:     cmd.Command,
			Description: cmd.Description,
		}
	}
	return out, nil
}

// DeleteBotCommands resets the bot command list for the given scope and language.
func (c *MCUBClient) DeleteBotCommands(ctx context.Context, scope string, langCode string) error {
	_, err := c.api.BotsResetBotCommands(ctx, &tg.BotsResetBotCommandsRequest{
		Scope:    botScopeFromString(scope),
		LangCode: langCode,
	})
	if err != nil {
		return fmt.Errorf("delete bot commands: %w", err)
	}
	return nil
}

// SetBotDescription sets the description text shown on the bot's profile page.
// langCode is the two-letter language code; empty means the default.
func (c *MCUBClient) SetBotDescription(ctx context.Context, description, langCode string) error {
	_, err := c.api.BotsSetBotInfo(ctx, &tg.BotsSetBotInfoRequest{
		Description: description,
		LangCode:    langCode,
	})
	if err != nil {
		return fmt.Errorf("set bot description: %w", err)
	}
	return nil
}

// SetBotAbout sets the bot's short description (the "about" text).
func (c *MCUBClient) SetBotAbout(ctx context.Context, about, langCode string) error {
	_, err := c.api.BotsSetBotInfo(ctx, &tg.BotsSetBotInfoRequest{
		About:    about,
		LangCode: langCode,
	})
	if err != nil {
		return fmt.Errorf("set bot about: %w", err)
	}
	return nil
}

// GetBotInfo returns the bot's name, description and about text for the given language.
func (c *MCUBClient) GetBotInfo(ctx context.Context, langCode string) (*BotInfo, error) {
	result, err := c.api.BotsGetBotInfo(ctx, &tg.BotsGetBotInfoRequest{
		LangCode: langCode,
	})
	if err != nil {
		return nil, fmt.Errorf("get bot info: %w", err)
	}
	return &BotInfo{
		Name:        result.Name,
		Description: result.Description,
		About:       result.About,
	}, nil
}

// GetBotCallbackAnswer presses an inline keyboard button and returns the bot's callback answer.
// This is the Telethon-MCUB–specific "send callback" helper (client-side, not bot-side).
func (c *MCUBClient) GetBotCallbackAnswer(ctx context.Context, chatID int64, msgID int, data []byte) (*tg.MessagesBotCallbackAnswer, error) {
	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("resolve peer: %w", err)
	}
	result, err := c.api.MessagesGetBotCallbackAnswer(ctx, &tg.MessagesGetBotCallbackAnswerRequest{
		Peer:  peer,
		MsgID: msgID,
		Data:  data,
	})
	if err != nil {
		return nil, fmt.Errorf("get bot callback answer: %w", err)
	}
	return result, nil
}

// botScopeFromString converts a scope name string to a tg.BotCommandScopeClass.
// Supported values: "", "default", "private", "groups", "group_admins", "all_private_chats",
// "all_group_chats", "all_chat_administrators". Unknown values fall back to the default scope.
func botScopeFromString(scope string) tg.BotCommandScopeClass {
	switch scope {
	case "private", "all_private_chats":
		return &tg.BotCommandScopeUsers{}
	case "groups", "all_group_chats":
		return &tg.BotCommandScopeChats{}
	case "group_admins", "all_chat_administrators":
		return &tg.BotCommandScopeChatAdmins{}
	default:
		return &tg.BotCommandScopeDefault{}
	}
}
