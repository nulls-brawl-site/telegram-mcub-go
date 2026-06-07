package types

// ChatGetter is embedded in message/event wrappers to provide chat info.
// It mirrors Telethon's tl/custom/chatgetter.ChatGetter base class.
type ChatGetter struct {
	ChatID    int64
	IsPrivate bool
	IsGroup   bool
	IsChannel bool
}

// GetChatID returns the chat ID.
func (c *ChatGetter) GetChatID() int64 {
	return c.ChatID
}

// IsPrivateChat reports whether the chat is a private (user) conversation.
func (c *ChatGetter) IsPrivateChat() bool {
	return c.IsPrivate
}

// IsGroupChat reports whether the chat is a group or supergroup.
func (c *ChatGetter) IsGroupChat() bool {
	return c.IsGroup
}

// IsChannelChat reports whether the chat is a broadcast channel.
func (c *ChatGetter) IsChannelChat() bool {
	return c.IsChannel
}
