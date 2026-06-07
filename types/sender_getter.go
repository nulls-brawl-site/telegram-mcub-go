package types

// SenderGetter is embedded in message/event wrappers to provide sender info.
// It mirrors Telethon's tl/custom/sendergetter.SenderGetter base class.
type SenderGetter struct {
	SenderID        int64
	SenderIsChannel bool
}

// GetSenderID returns the sender's ID.
func (s *SenderGetter) GetSenderID() int64 {
	return s.SenderID
}

// IsFromUser reports whether the sender is a regular user (not a channel).
func (s *SenderGetter) IsFromUser() bool {
	return s.SenderID != 0 && !s.SenderIsChannel
}

// IsFromChannel reports whether the message originated from a channel/broadcast.
func (s *SenderGetter) IsFromChannel() bool {
	return s.SenderIsChannel
}

// IsFromGroup reports whether the sender is acting within a group context
// (i.e. has a positive numeric user ID and is not a channel).
func (s *SenderGetter) IsFromGroup() bool {
	return s.SenderID != 0 && !s.SenderIsChannel
}
