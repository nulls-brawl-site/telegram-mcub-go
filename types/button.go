package types

import "github.com/gotd/td/tg"

// ButtonStyle defines the visual style of an inline button (MCUB extension).
type ButtonStyle int

const (
	ButtonStyleDefault ButtonStyle = iota
	ButtonStylePrimary
	ButtonStyleSuccess
	ButtonStyleDanger
)

func (s ButtonStyle) String() string {
	switch s {
	case ButtonStylePrimary:
		return "primary"
	case ButtonStyleSuccess:
		return "success"
	case ButtonStyleDanger:
		return "danger"
	default:
		return "default"
	}
}

// Button is an MCUB-extended button definition.
type Button struct {
	// Text is the visible label of the button.
	Text string

	// Style is the visual style (MCUB extension; encoded via emoji prefix when supported).
	Style ButtonStyle

	// Icon is an optional custom emoji ID used as a button icon.
	Icon string

	// Data is the callback data for inline buttons.
	Data []byte

	// URL is used for URL buttons.
	URL string

	// SwitchInlineQuery is set for switch-inline-query buttons.
	SwitchInlineQuery *string

	// SwitchInlineQueryCurrentChat is set for switch-inline-query-current-chat buttons.
	SwitchInlineQueryCurrentChat *string
}

// ToTL converts the Button to a gotd/td TL keyboard button.
func (b *Button) ToTL() tg.KeyboardButtonClass {
	text := b.styledText()

	if b.URL != "" {
		return &tg.KeyboardButtonURL{
			Text: text,
			URL:  b.URL,
		}
	}
	if b.SwitchInlineQuery != nil {
		return &tg.KeyboardButtonSwitchInline{
			Text:  text,
			Query: *b.SwitchInlineQuery,
		}
	}
	if b.SwitchInlineQueryCurrentChat != nil {
		return &tg.KeyboardButtonSwitchInline{
			Text:      text,
			Query:     *b.SwitchInlineQueryCurrentChat,
			SamePeer:  true,
		}
	}
	return &tg.KeyboardButtonCallback{
		Text: text,
		Data: b.Data,
	}
}

// styledText prepends a style indicator to the button text.
// Telegram doesn't natively support button styles, so MCUB uses emoji prefixes.
func (b *Button) styledText() string {
	prefix := ""
	switch b.Style {
	case ButtonStylePrimary:
		prefix = "🔵 "
	case ButtonStyleSuccess:
		prefix = "✅ "
	case ButtonStyleDanger:
		prefix = "🔴 "
	}
	return prefix + b.Text
}

// ButtonRow is a horizontal row of buttons.
type ButtonRow []*Button

// ButtonGrid is a 2-D grid of buttons (rows of rows).
type ButtonGrid []ButtonRow

// ToTLMarkup converts the ButtonGrid to a tg.ReplyInlineMarkup.
func (g ButtonGrid) ToTLMarkup() *tg.ReplyInlineMarkup {
	rows := make([]tg.KeyboardButtonRow, 0, len(g))
	for _, row := range g {
		tlRow := tg.KeyboardButtonRow{}
		for _, btn := range row {
			tlRow.Buttons = append(tlRow.Buttons, btn.ToTL())
		}
		rows = append(rows, tlRow)
	}
	return &tg.ReplyInlineMarkup{Rows: rows}
}

// NewButtonRow is a convenience constructor for a single-row grid.
func NewButtonRow(buttons ...*Button) ButtonGrid {
	return ButtonGrid{ButtonRow(buttons)}
}

// NewButton creates a simple callback button.
func NewButton(text string, data []byte) *Button {
	return &Button{Text: text, Data: data, Style: ButtonStyleDefault}
}

// NewURLButton creates a URL button.
func NewURLButton(text, url string) *Button {
	return &Button{Text: text, URL: url}
}

// NewPrimaryButton creates a primary-styled callback button.
func NewPrimaryButton(text string, data []byte) *Button {
	return &Button{Text: text, Data: data, Style: ButtonStylePrimary}
}

// NewSuccessButton creates a success-styled callback button.
func NewSuccessButton(text string, data []byte) *Button {
	return &Button{Text: text, Data: data, Style: ButtonStyleSuccess}
}

// NewDangerButton creates a danger-styled callback button.
func NewDangerButton(text string, data []byte) *Button {
	return &Button{Text: text, Data: data, Style: ButtonStyleDanger}
}

// ---------------------------------------------------------------------------
// MessageButton — parsed button from an incoming message
// ---------------------------------------------------------------------------

// MessageButton represents a button already attached to a received message.
// It is read-only; use Button to build new markup for outgoing messages.
type MessageButton struct {
	// Text is the visible button label.
	Text string

	// Data is the callback payload (inline buttons only).
	Data []byte

	// URL is the destination URL (URL buttons only).
	URL string

	// Style is the visual style, if detectable.
	Style ButtonStyle

	// Icon is the custom emoji document ID (MCUB extension), or 0.
	Icon int64
}

// BuildMarkup converts a ButtonGrid to a tg.ReplyMarkupClass suitable for
// sending in a message.
func BuildMarkup(rows [][]Button) tg.ReplyMarkupClass {
	tlRows := make([]tg.KeyboardButtonRow, 0, len(rows))
	for _, row := range rows {
		tlRow := tg.KeyboardButtonRow{}
		for i := range row {
			tlRow.Buttons = append(tlRow.Buttons, row[i].ToTL())
		}
		tlRows = append(tlRows, tlRow)
	}
	return &tg.ReplyInlineMarkup{Rows: tlRows}
}

// ParseMarkup parses a tg.ReplyMarkupClass back into a grid of MessageButtons.
// Returns nil when markup is nil or contains no buttons.
func ParseMarkup(markup tg.ReplyMarkupClass) [][]*MessageButton {
	if markup == nil {
		return nil
	}

	var rows []tg.KeyboardButtonRow
	switch m := markup.(type) {
	case *tg.ReplyInlineMarkup:
		rows = m.Rows
	case *tg.ReplyKeyboardMarkup:
		rows = m.Rows
	default:
		return nil
	}

	if len(rows) == 0 {
		return nil
	}

	grid := make([][]*MessageButton, 0, len(rows))
	for _, row := range rows {
		r := make([]*MessageButton, 0, len(row.Buttons))
		for _, btn := range row.Buttons {
			mb := parseButton(btn)
			if mb != nil {
				r = append(r, mb)
			}
		}
		grid = append(grid, r)
	}
	return grid
}

// parseButton converts a single tg.KeyboardButtonClass into a MessageButton.
func parseButton(btn tg.KeyboardButtonClass) *MessageButton {
	if btn == nil {
		return nil
	}
	switch b := btn.(type) {
	case *tg.KeyboardButtonCallback:
		return &MessageButton{Text: b.Text, Data: b.Data}
	case *tg.KeyboardButtonURL:
		return &MessageButton{Text: b.Text, URL: b.URL}
	case *tg.KeyboardButton:
		return &MessageButton{Text: b.Text}
	case *tg.KeyboardButtonSwitchInline:
		return &MessageButton{Text: b.Text}
	case *tg.KeyboardButtonBuy:
		return &MessageButton{Text: b.Text}
	case *tg.KeyboardButtonGame:
		return &MessageButton{Text: b.Text}
	default:
		// Best-effort: try to get the text via the ButtonText interface.
		type texted interface{ GetText() string }
		if t, ok := btn.(texted); ok {
			return &MessageButton{Text: t.GetText()}
		}
		return nil
	}
}
