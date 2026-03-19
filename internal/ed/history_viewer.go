package ed

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// historyViewer opens a full-screen ANSI overlay showing the command history.
// The user can navigate with arrows, search with /, delete with Del/d,
// and press Enter to select an entry. Returns the selected line or "" if cancelled.
func (ed *Editor) historyViewer(reader *bufio.Reader) string {
	items := ed.history
	if len(items) == 0 {
		return ""
	}

	cols := getTermCols()
	rows := getTermRows()

	// Colours
	const (
		reset   = "\x1b[0m"
		titleBg = "\x1b[48;5;24m\x1b[97m"  // deep blue bg, white text
		selBg   = "\x1b[48;5;237m\x1b[97m"  // dark grey bg, white text
		searchC = "\x1b[48;5;22m\x1b[97m"   // dark green bg, white text
		statusC = "\x1b[48;5;238m\x1b[37m"  // grey bg, light text
		dimC    = "\x1b[38;5;243m"           // dim grey for non-selected items
		matchC  = "\x1b[33m"                 // yellow for search matches
		idxC    = "\x1b[38;5;243m"           // dim for line numbers
	)

	// State
	filtered := make([]int, len(items)) // indices into items
	for i := range items {
		filtered[i] = i
	}
	cursor := len(filtered) - 1 // start at most recent
	searchMode := false
	searchBuf := []rune{}
	deleted := make(map[int]bool) // indices into items that are marked for deletion

	applyFilter := func() {
		query := strings.ToLower(string(searchBuf))
		filtered = filtered[:0]
		for i, item := range items {
			if deleted[i] {
				continue
			}
			if query == "" || strings.Contains(strings.ToLower(item), query) {
				filtered = append(filtered, i)
			}
		}
		if cursor >= len(filtered) {
			cursor = len(filtered) - 1
		}
		if cursor < 0 {
			cursor = 0
		}
	}

	// Switch to alternate screen, hide cursor
	os.Stdout.WriteString("\x1b[?1049h\x1b[H\x1b[2J\x1b[?25l")

	render := func() {
		// Available rows: total - 2 (title bar + status/search bar)
		listRows := rows - 2
		if listRows < 1 {
			listRows = 1
		}

		// Compute scroll offset to keep cursor visible
		scrollOff := 0
		if cursor >= listRows {
			scrollOff = cursor - listRows + 1
		}

		var b strings.Builder

		// Move to top, clear screen
		b.WriteString("\x1b[H\x1b[2J")

		// Title bar
		title := fmt.Sprintf(" History (%d entries)", len(filtered))
		if len(searchBuf) > 0 && !searchMode {
			title += fmt.Sprintf("  filter: %s", string(searchBuf))
		}
		pad := cols - len(title)
		if pad > 0 {
			title += strings.Repeat(" ", pad)
		}
		b.WriteString(titleBg)
		b.WriteString(title[:min(len(title), cols)])
		b.WriteString(reset)
		b.WriteString("\r\n")

		query := strings.ToLower(string(searchBuf))

		// List
		for row := 0; row < listRows; row++ {
			idx := scrollOff + row
			if idx < len(filtered) {
				origIdx := filtered[idx]
				line := items[origIdx]
				maxW := cols - 6 // room for prefix + number gutter
				if maxW < 1 {
					maxW = 1
				}
				display := line
				if len(display) > maxW {
					display = display[:maxW-1] + "…"
				}

				if idx == cursor {
					// Selected row: highlighted background
					b.WriteString(selBg)
					b.WriteString(fmt.Sprintf(" %3d ", origIdx+1))
					// Highlight search matches within selected line
					if query != "" {
						b.WriteString(highlightMatch(display, query, matchC, selBg))
					} else {
						b.WriteString(display)
					}
					trailing := cols - 5 - len(display)
					if trailing > 0 {
						b.WriteString(strings.Repeat(" ", trailing))
					}
					b.WriteString(reset)
				} else {
					// Normal row
					b.WriteString(idxC)
					b.WriteString(fmt.Sprintf(" %3d ", origIdx+1))
					b.WriteString(reset)
					if query != "" {
						b.WriteString(highlightMatch(display, query, matchC, dimC))
					} else {
						b.WriteString(dimC)
						b.WriteString(display)
						b.WriteString(reset)
					}
				}
			}
			b.WriteString("\r\n")
		}

		// Status / search bar
		if searchMode {
			status := fmt.Sprintf(" /%s", string(searchBuf))
			pad := cols - len(status)
			if pad > 0 {
				status += strings.Repeat(" ", pad)
			}
			b.WriteString(searchC)
			b.WriteString(status[:min(len(status), cols)])
			b.WriteString(reset)
		} else {
			status := " ↑↓ navigate  / search  Enter select  d delete  Esc quit"
			pad := cols - len(status)
			if pad > 0 {
				status += strings.Repeat(" ", pad)
			}
			b.WriteString(statusC)
			if len(status) > cols {
				status = status[:cols]
			}
			b.WriteString(status)
			b.WriteString(reset)
		}

		os.Stdout.WriteString(b.String())
	}

	deleteCurrent := func() {
		if len(filtered) == 0 {
			return
		}
		origIdx := filtered[cursor]
		deleted[origIdx] = true
		applyFilter()
		render()
	}

	// isEscSeq checks if there are buffered bytes after ESC (meaning it's
	// an escape sequence, not a bare Esc keypress).
	isEscSeq := func() bool {
		return reader.Buffered() > 0
	}

	render()

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			break
		}

		if searchMode {
			switch {
			case r == 0x1b: // Esc — exit search, back to navigation with filter intact
				searchMode = false
				render()
			case r == '\r' || r == '\n': // confirm search, back to navigation
				searchMode = false
				render()
			case r == 127: // backspace in search
				if len(searchBuf) > 0 {
					searchBuf = searchBuf[:len(searchBuf)-1]
					applyFilter()
				}
				render()
			case r == 21: // Ctrl+U — clear search
				searchBuf = searchBuf[:0]
				applyFilter()
				render()
			case r >= 32: // printable
				searchBuf = append(searchBuf, r)
				applyFilter()
				render()
			}
			continue
		}

		// Navigation mode
		switch {
		case r == 0x1b: // ESC — check if sequence or bare
			if !isEscSeq() {
				// Bare Esc — cancel and exit
				goto done
			}
			r1, _, err := reader.ReadRune()
			if err != nil {
				goto done
			}
			if r1 == '[' {
				r2, _, err := reader.ReadRune()
				if err != nil {
					continue
				}
				switch r2 {
				case 'A': // up
					if cursor > 0 {
						cursor--
					}
				case 'B': // down
					if cursor < len(filtered)-1 {
						cursor++
					}
				case '3': // Del — ESC [ 3 ~
					reader.ReadRune() // consume '~'
					deleteCurrent()
					continue
				case '5': // PgUp — ESC [ 5 ~
					reader.ReadRune() // consume '~'
					cursor -= (rows - 2)
					if cursor < 0 {
						cursor = 0
					}
				case '6': // PgDn — ESC [ 6 ~
					reader.ReadRune() // consume '~'
					cursor += (rows - 2)
					if cursor >= len(filtered) {
						cursor = len(filtered) - 1
					}
				case 'H': // Home
					cursor = 0
				case 'F': // End
					cursor = len(filtered) - 1
				default:
					// Drain unknown CSI sequence
					for reader.Buffered() > 0 {
						b, _ := reader.Peek(1)
						if len(b) > 0 && b[0] >= 0x40 && b[0] <= 0x7e {
							reader.ReadRune()
							break
						}
						reader.ReadRune()
					}
				}
			} else if r1 == 'O' {
				r2, _, _ := reader.ReadRune()
				if r2 == 'H' {
					cursor = 0
				} else if r2 == 'F' {
					cursor = len(filtered) - 1
				}
			}
			render()

		case r == '\r' || r == '\n': // Enter — select
			if len(filtered) > 0 && cursor >= 0 && cursor < len(filtered) {
				selected := items[filtered[cursor]]
				commitDeletions(ed, deleted)
				os.Stdout.WriteString("\x1b[?25h\x1b[?1049l")
				return selected
			}
			goto done

		case r == '/': // start search
			searchMode = true
			render()

		case r == 'q', r == 8: // q or Ctrl+H — cancel
			goto done

		case r == 'd': // delete current entry
			deleteCurrent()

		case r == 'k': // vim up
			if cursor > 0 {
				cursor--
			}
			render()
		case r == 'j': // vim down
			if cursor < len(filtered)-1 {
				cursor++
			}
			render()
		case r == 'g': // vim top
			cursor = 0
			render()
		case r == 'G': // vim bottom
			cursor = len(filtered) - 1
			render()
		}
	}

done:
	commitDeletions(ed, deleted)
	// Restore main screen, show cursor
	os.Stdout.WriteString("\x1b[?25h\x1b[?1049l")
	return ""
}

// commitDeletions removes marked entries from history in-memory and rewrites the history file.
func commitDeletions(ed *Editor, deleted map[int]bool) {
	if len(deleted) == 0 {
		return
	}
	newHist := make([]string, 0, len(ed.history)-len(deleted))
	for i, item := range ed.history {
		if !deleted[i] {
			newHist = append(newHist, item)
		}
	}
	ed.history = newHist

	// Rewrite history file
	f, err := os.Create(ed.histPath)
	if err != nil {
		return
	}
	defer f.Close()
	for _, line := range ed.history {
		fmt.Fprintln(f, line)
	}
}

// highlightMatch highlights case-insensitive occurrences of query in text.
func highlightMatch(text, query, hlColor, baseColor string) string {
	lower := strings.ToLower(text)
	var b strings.Builder
	i := 0
	for i < len(text) {
		idx := strings.Index(lower[i:], query)
		if idx < 0 {
			b.WriteString(baseColor)
			b.WriteString(text[i:])
			b.WriteString("\x1b[0m")
			break
		}
		if idx > 0 {
			b.WriteString(baseColor)
			b.WriteString(text[i : i+idx])
		}
		b.WriteString(hlColor)
		b.WriteString(text[i+idx : i+idx+len(query)])
		b.WriteString("\x1b[0m")
		i += idx + len(query)
	}
	return b.String()
}

// getTermRows returns the terminal height, falling back to 24.
func getTermRows() int {
	rows := 24
	var ws struct{ Row, Col, X, Y uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdout), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno == 0 && ws.Row > 0 {
		rows = int(ws.Row)
	}
	return rows
}
