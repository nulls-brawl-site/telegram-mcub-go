package types

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/gotd/td/tg"
)

// InlineBuilder builds inline query result objects for answering inline queries.
// It mirrors Telethon's tl/custom/inlinebuilder.InlineBuilder class.
type InlineBuilder struct {
	// botID is the bot's user ID (used for deterministic ID generation).
	botID int64
}

// NewInlineBuilder creates a new InlineBuilder for the given bot.
func NewInlineBuilder(botID int64) *InlineBuilder {
	return &InlineBuilder{botID: botID}
}

// ArticleParams holds parameters for building an article inline result.
type ArticleParams struct {
	// ID is the result ID.  If empty, a SHA-256 of the content is used.
	ID string
	// Title is the article title.
	Title string
	// Description is the article description shown below the title.
	Description string
	// Text is the message text that will be sent when the user selects the result.
	Text string
	// ParseMode controls formatting ("HTML", "Markdown", or "").
	ParseMode string
	// URL is an optional article URL.
	URL string
	// Thumb is an optional thumbnail.
	Thumb *ThumbParams
	// Buttons is the optional keyboard attached to the sent message.
	Buttons [][]tg.KeyboardButtonClass
	// LinkPreview enables or disables link previews in the sent message.
	LinkPreview bool
}

// ThumbParams holds thumbnail parameters for inline results.
type ThumbParams struct {
	URL      string
	Width    int
	Height   int
	MimeType string
}

// PhotoResultParams holds parameters for building a photo inline result.
type PhotoResultParams struct {
	// ID is the result ID.  If empty, a deterministic SHA-256 is used.
	ID       string
	URL      string
	ThumbURL string
	Title    string
	// ParseMode controls caption formatting.
	ParseMode string
	// Text is the caption sent with the photo.
	Text    string
	Width   int
	Height  int
	Buttons [][]tg.KeyboardButtonClass
}

// DocumentResultParams holds parameters for building a document inline result.
type DocumentResultParams struct {
	// ID is the result ID.  If empty, a deterministic SHA-256 is used.
	ID        string
	URL       string
	Title     string
	MimeType  string
	ParseMode string
	// Text is the caption sent with the document.
	Text    string
	Buttons [][]tg.KeyboardButtonClass
}

// Article creates a text article inline result.
func (b *InlineBuilder) Article(params ArticleParams) tg.InputBotInlineResultClass {
	markup := buildInlineMarkup(params.Buttons)

	sendMsg := &tg.InputBotInlineMessageText{
		Message:     params.Text,
		NoWebpage:   !params.LinkPreview,
		ReplyMarkup: markup,
	}

	result := &tg.InputBotInlineResult{
		ID:          params.ID,
		Type:        "article",
		Title:       params.Title,
		Description: params.Description,
		URL:         params.URL,
		SendMessage: sendMsg,
	}
	if params.Thumb != nil {
		result.SetThumb(tg.InputWebDocument{
			URL:      params.Thumb.URL,
			Size:     0,
			MimeType: params.Thumb.MimeType,
		})
	}

	if result.ID == "" {
		result.ID = deterministicID(params.Title + params.Text + params.URL)
	}

	return result
}

// Photo creates a photo inline result from a web URL.
func (b *InlineBuilder) Photo(params PhotoResultParams) tg.InputBotInlineResultClass {
	markup := buildInlineMarkup(params.Buttons)

	sendMsg := &tg.InputBotInlineMessageMediaAuto{
		Message:     params.Text,
		ReplyMarkup: markup,
	}

	result := &tg.InputBotInlineResult{
		ID:          params.ID,
		Type:        "photo",
		Title:       params.Title,
		URL:         params.URL,
		SendMessage: sendMsg,
	}
	if params.ThumbURL != "" {
		result.SetThumb(tg.InputWebDocument{
			URL:      params.ThumbURL,
			MimeType: "image/jpeg",
		})
	}

	if result.ID == "" {
		result.ID = deterministicID(params.URL + params.Text)
	}

	return result
}

// Document creates a document inline result from a web URL.
func (b *InlineBuilder) Document(params DocumentResultParams) tg.InputBotInlineResultClass {
	markup := buildInlineMarkup(params.Buttons)

	sendMsg := &tg.InputBotInlineMessageMediaAuto{
		Message:     params.Text,
		ReplyMarkup: markup,
	}

	result := &tg.InputBotInlineResult{
		ID:          params.ID,
		Type:        "file",
		Title:       params.Title,
		URL:         params.URL,
		SendMessage: sendMsg,
	}

	if result.ID == "" {
		result.ID = deterministicID(params.URL + params.Title)
	}

	return result
}

// Game creates a game inline result for the given short name.
func (b *InlineBuilder) Game(shortName string) tg.InputBotInlineResultClass {
	return &tg.InputBotInlineResultGame{
		ID:        deterministicID(shortName),
		ShortName: shortName,
		SendMessage: &tg.InputBotInlineMessageGame{},
	}
}

// buildInlineMarkup converts a 2-D slice of KeyboardButtonClass to a
// ReplyInlineMarkup (nil when buttons is empty).
func buildInlineMarkup(buttons [][]tg.KeyboardButtonClass) tg.ReplyMarkupClass {
	if len(buttons) == 0 {
		return nil
	}
	rows := make([]tg.KeyboardButtonRow, 0, len(buttons))
	for _, row := range buttons {
		rows = append(rows, tg.KeyboardButtonRow{Buttons: row})
	}
	return &tg.ReplyInlineMarkup{Rows: rows}
}

// deterministicID returns a short hex string derived from s.
func deterministicID(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}
