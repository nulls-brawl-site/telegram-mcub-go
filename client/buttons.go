package client

import (
	"fmt"

	"github.com/gotd/td/tg"
)

// ButtonGrid is a helper for building keyboard button grids incrementally.
type ButtonGrid struct {
	Rows [][]tg.KeyboardButtonClass
}

// NewButtonGrid creates an empty ButtonGrid.
func NewButtonGrid() *ButtonGrid {
	return &ButtonGrid{}
}

// AddRow appends a new row of buttons to the grid.
func (bg *ButtonGrid) AddRow(buttons ...tg.KeyboardButtonClass) *ButtonGrid {
	row := make([]tg.KeyboardButtonClass, len(buttons))
	copy(row, buttons)
	bg.Rows = append(bg.Rows, row)
	return bg
}

// Build converts the grid into a tg.ReplyMarkupClass.
// If any button in the grid is an inline button, a ReplyInlineMarkup is
// returned; otherwise a ReplyKeyboardMarkup is returned.
func (bg *ButtonGrid) Build() tg.ReplyMarkupClass {
	rows := make([]tg.KeyboardButtonRow, len(bg.Rows))
	inline := false
	for i, row := range bg.Rows {
		rows[i] = tg.KeyboardButtonRow{Buttons: row}
		for _, btn := range row {
			if isInlineButton(btn) {
				inline = true
			}
		}
	}
	if inline {
		return &tg.ReplyInlineMarkup{Rows: rows}
	}
	return &tg.ReplyKeyboardMarkup{Rows: rows}
}

// BuildButton creates a tg.KeyboardButtonClass from various input types:
//   - tg.KeyboardButtonClass — returned as-is
//   - string — creates a plain KeyboardButton with that text
//   - map[string]string with key "text" — plain button; if also has "url", creates
//     an inline URL button; if "data", creates a callback button
func BuildButton(btn interface{}) (tg.KeyboardButtonClass, error) {
	switch v := btn.(type) {
	case tg.KeyboardButtonClass:
		return v, nil
	case string:
		return &tg.KeyboardButton{Text: v}, nil
	case map[string]string:
		text := v["text"]
		if text == "" {
			return nil, fmt.Errorf("button map missing 'text' key")
		}
		if url, ok := v["url"]; ok && url != "" {
			return &tg.KeyboardButtonURL{Text: text, URL: url}, nil
		}
		if data, ok := v["data"]; ok {
			return &tg.KeyboardButtonCallback{Text: text, Data: []byte(data)}, nil
		}
		if switchInline, ok := v["switch_inline"]; ok {
			return &tg.KeyboardButtonSwitchInline{Text: text, Query: switchInline}, nil
		}
		return &tg.KeyboardButton{Text: text}, nil
	case map[string]interface{}:
		text, _ := v["text"].(string)
		if text == "" {
			return nil, fmt.Errorf("button map missing 'text' key")
		}
		if url, _ := v["url"].(string); url != "" {
			return &tg.KeyboardButtonURL{Text: text, URL: url}, nil
		}
		if data, _ := v["data"].([]byte); data != nil {
			return &tg.KeyboardButtonCallback{Text: text, Data: data}, nil
		}
		if dataStr, _ := v["data"].(string); dataStr != "" {
			return &tg.KeyboardButtonCallback{Text: text, Data: []byte(dataStr)}, nil
		}
		if q, _ := v["switch_inline"].(string); q != "" {
			return &tg.KeyboardButtonSwitchInline{Text: text, Query: q}, nil
		}
		return &tg.KeyboardButton{Text: text}, nil
	default:
		return nil, fmt.Errorf("unsupported button type %T", btn)
	}
}

// BuildReplyMarkup converts a flexible button layout to a tg.ReplyMarkupClass.
//
// The buttons argument may be:
//   - nil → returns nil
//   - tg.ReplyMarkupClass → returned as-is
//   - tg.KeyboardButtonClass → single-button single-row markup
//   - []tg.KeyboardButtonClass → single-row markup
//   - [][]tg.KeyboardButtonClass → full grid
//   - []interface{} → single row where each element is passed to BuildButton
//   - [][]interface{} → full grid where each element is passed to BuildButton
func BuildReplyMarkup(buttons interface{}) (tg.ReplyMarkupClass, error) {
	if buttons == nil {
		return nil, nil
	}

	// Already a markup.
	if m, ok := buttons.(tg.ReplyMarkupClass); ok {
		return m, nil
	}

	// Normalise to [][]interface{}.
	var grid [][]interface{}

	switch v := buttons.(type) {
	case tg.KeyboardButtonClass:
		grid = [][]interface{}{{v}}

	case []tg.KeyboardButtonClass:
		row := make([]interface{}, len(v))
		for i, b := range v {
			row[i] = b
		}
		grid = [][]interface{}{row}

	case [][]tg.KeyboardButtonClass:
		grid = make([][]interface{}, len(v))
		for i, row := range v {
			gr := make([]interface{}, len(row))
			for j, b := range row {
				gr[j] = b
			}
			grid[i] = gr
		}

	case []interface{}:
		// Check if first element is itself a slice → treat as row-of-rows.
		if len(v) > 0 {
			if _, isSlice := v[0].([]interface{}); isSlice {
				grid = make([][]interface{}, len(v))
				for i, rowRaw := range v {
					if row, ok := rowRaw.([]interface{}); ok {
						grid[i] = row
					} else {
						grid[i] = []interface{}{rowRaw}
					}
				}
			} else {
				grid = [][]interface{}{v}
			}
		}

	case [][]interface{}:
		grid = v

	default:
		return nil, fmt.Errorf("unsupported buttons type %T", buttons)
	}

	bg := NewButtonGrid()
	for _, row := range grid {
		var rowBtns []tg.KeyboardButtonClass
		for _, raw := range row {
			btn, err := BuildButton(raw)
			if err != nil {
				return nil, err
			}
			rowBtns = append(rowBtns, btn)
		}
		if len(rowBtns) > 0 {
			bg.AddRow(rowBtns...)
		}
	}
	return bg.Build(), nil
}

// IsInline returns true if any button in the layout is an inline button
// (URL, callback, switch-inline, etc.).
func IsInline(buttons [][]interface{}) bool {
	for _, row := range buttons {
		for _, raw := range row {
			if btn, ok := raw.(tg.KeyboardButtonClass); ok {
				if isInlineButton(btn) {
					return true
				}
			}
		}
	}
	return false
}

// isInlineButton reports whether a button belongs to the inline markup family.
func isInlineButton(btn tg.KeyboardButtonClass) bool {
	switch btn.(type) {
	case *tg.KeyboardButtonCallback,
		*tg.KeyboardButtonURL,
		*tg.KeyboardButtonSwitchInline,
		*tg.KeyboardButtonGame,
		*tg.KeyboardButtonBuy,
		*tg.KeyboardButtonURLAuth:
		return true
	}
	return false
}
