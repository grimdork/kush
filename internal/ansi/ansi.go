package ansi

import "fmt"

// ClearLine returns an ANSI sequence that clears the current line.
func ClearLine() string {
	return "\x1b[2K"
}

// MoveCursorUp returns an ANSI sequence to move the cursor up n lines.
func MoveCursorUp(n int) string {
	return fmt.Sprintf("\x1b[%dA", n)
}

// MoveCursorTo returns an ANSI sequence to move cursor to row,col (1-indexed).
func MoveCursorTo(row, col int) string {
	return fmt.Sprintf("\x1b[%d;%dH", row, col)
}

// SaveCursor returns the sequence to save the cursor position.
func SaveCursor() string { return "\x1b7" }

// RestoreCursor returns the sequence to restore the cursor position.
func RestoreCursor() string { return "\x1b8" }

// Bold wraps a string in ANSI bold on/off markers.
func Bold(s string) string { return "\x1b[1m" + s + "\x1b[22m" }
