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
