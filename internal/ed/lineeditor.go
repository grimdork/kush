package ed

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"github.com/grimdork/kush/internal/completion"
	"github.com/grimdork/kush/internal/log"
)

// ErrEOF is returned when the user signals end-of-input (e.g. Ctrl+D).
var ErrEOF = errors.New("eof")

// Editor provides a minimal ANSI/raw-mode line editor that doesn't use a
// fullscreen library. It edits a single line, supports left/right, backspace,
// simple history and tab as literal.
type Editor struct {
	history  []string
	histPath string
	// completion state
	compStart      int
	compCandidates []string
	compIndex      int
	compPageStart  int
	// disable DECSTBM usage for this session after observed failures
	disableDECSTBM bool
	// inCompletion indicates we have active candidate rows displayed and
	// subsequent Tab presses should reuse them rather than reflow/print above.
	inCompletion bool
}

// Close is a no-op for the simple ANSI editor (kept for API compatibility).
func (ed *Editor) Close() {
	// nothing to do; ensure terminal restored if necessary elsewhere
}

// New creates the editor and loads history.
func New() (*Editor, error) {
	home := os.Getenv("HOME")
	hist := filepath.Join(home, ".kush_history")
	history := []string{}
	if data, err := os.ReadFile(hist); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(l) != "" {
				history = append(history, l)
			}
		}
	}
	ed := &Editor{history: history, histPath: hist}
	if os.Getenv("KUSH_DISABLE_DECSTBM") == "1" {
		ed.disableDECSTBM = true
	}
	return ed, nil
}

// appendHistory saves a new entry to the history in-memory and on disk.
func (ed *Editor) appendHistory(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	ed.history = append(ed.history, line)
	f, err := os.OpenFile(ed.histPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line + "\n")
}

var ansiRE = regexp.MustCompile("\\x1b\\[[0-9;]*[a-zA-Z]")

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// ensureCursor moves the terminal cursor to the absolute column corresponding
// to len(stripANSI(prompt)) + cursor and optionally logs debug info.
func ensureCursor(prompt string, buf []rune, cursor int) {
	visiblePrompt := stripANSI(prompt)
	col := len([]rune(visiblePrompt)) + cursor + 1 // CSI G is 1-based
	os.Stdout.WriteString("\r")
	os.Stdout.WriteString("\x1b[" + fmt.Sprintf("%d", col) + "G")
	if os.Getenv("KUSH_KEYDEBUG") == "2" || os.Getenv("KUSH_KEYDEBUG") == "3" {
		fmt.Fprintf(os.Stderr, "CURDEBUG visiblePrompt=%q promptLen=%d cursor=%d col=%d\n", visiblePrompt, len([]rune(visiblePrompt)), cursor, col)
	}
}

// renderLine redraws the prompt and buffer, positioning cursor.
func renderLine(prompt string, buf []rune, cursor int) {
	// carriage return, clear line, write prompt+buffer, move cursor
	os.Stdout.WriteString("\r\x1b[K")
	os.Stdout.WriteString(prompt)
	os.Stdout.WriteString(string(buf))
	// move cursor to position after prompt+cursor
	// Use visible prompt width (stripANSI) so ANSI colouring doesn't confuse column math
	visiblePrompt := stripANSI(prompt)
	col := len([]rune(visiblePrompt)) + cursor + 1 // CSI G is 1-based
	// move to column col: write \r then absolute column (CSI nG)
	os.Stdout.WriteString("\r")
	os.Stdout.WriteString("\x1b[" + fmt.Sprintf("%d", col) + "G")
}

// getTermCols returns the terminal width, falling back to $COLUMNS then 80.
func getTermCols() int {
	cols := 80
	var ws struct{ Row, Col, X, Y uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(syscall.Stdout), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno == 0 && ws.Col > 0 {
		cols = int(ws.Col)
	} else if v := os.Getenv("COLUMNS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cols = n
		}
	}
	return cols
}

// compLayout computes the column width, candidates per line, visible slots,
// page start/end for the current candidate list and terminal width.
func (ed *Editor) compLayout() (colw, perLine, visible, start, end int) {
	cands := ed.compCandidates
	cols := getTermCols()
	maxw := 0
	for _, c := range cands {
		if w := len(c); w > maxw {
			maxw = w
		}
	}
	if maxw == 0 {
		maxw = 1
	}
	colw = maxw + 2
	perLine = cols / colw
	if perLine < 1 {
		perLine = 1
	}
	visible = perLine * 2
	// normalise page start
	if visible > 0 {
		ed.compPageStart = (ed.compPageStart / visible) * visible
	}
	if ed.compPageStart < 0 {
		ed.compPageStart = 0
	}
	if ed.compPageStart >= len(cands) {
		ed.compPageStart = 0
	}
	start = ed.compPageStart
	end = start + visible
	if end > len(cands) {
		end = len(cands)
	}
	return
}

// buildCandidateRow builds a single row string of candidates from indices
// [from, from+perLine) within [start, end), highlighting compIndex.
func (ed *Editor) buildCandidateRow(from, perLine, end, colw int) string {
	cands := ed.compCandidates
	var row bytes.Buffer
	for idx := 0; idx < perLine; idx++ {
		i := from + idx
		if i < end {
			s := cands[i]
			if i == ed.compIndex {
				row.WriteString(colWrap(s, true))
			} else {
				row.WriteString(s)
			}
			pad := colw - len(s)
			for p := 0; p < pad; p++ {
				row.WriteByte(' ')
			}
		}
	}
	return row.String()
}

// resetCompletion clears all completion state and the inCompletion flag.
func (ed *Editor) resetCompletion() {
	ed.compCandidates = nil
	ed.compIndex = 0
	ed.compPageStart = 0
	ed.inCompletion = false
}

// longestCommonPrefix returns the longest common prefix of all candidates.
func longestCommonPrefix(cands []string) string {
	if len(cands) == 0 {
		return ""
	}
	prefix := cands[0]
	for _, c := range cands[1:] {
		for i := 0; i < len(prefix) && i < len(c); i++ {
			if prefix[i] != c[i] {
				prefix = prefix[:i]
				break
			}
		}
		if len(c) < len(prefix) {
			prefix = prefix[:len(c)]
		}
	}
	return prefix
}

// renderCandidates renders two rows of candidates and a prompt line below them.
//
// Layout (all downward from the current cursor line):
//
//	current line → candidate row 1
//	next line    → candidate row 2
//	next line    → prompt + preview of selected candidate
//
// When inCompletion is already set (subsequent Tab presses) the cursor is on
// the prompt line (2 below candidates). We move up 2, redraw the rows, move
// back down and redraw the prompt.
func (ed *Editor) renderCandidates(prompt string, buf []rune, cursor int) {
	cands := ed.compCandidates
	if len(cands) == 0 {
		return
	}

	colw, perLine, _, start, end := ed.compLayout()
	keyDebug := os.Getenv("KUSH_KEYDEBUG")

	b := &bytes.Buffer{}

	if ed.inCompletion {
		// Cursor is on the prompt line, 2 rows below the candidate block.
		// Move up 2 to redraw the candidate rows in place.
		b.WriteString("\x1b[2A")
	}
	// else: cursor is on the original prompt line which becomes candidate row 1.

	// Row 1: clear line, write candidates
	b.WriteString("\r\x1b[2K")
	b.WriteString(ed.buildCandidateRow(start, perLine, end, colw))

	// Row 2: move down, clear line, write candidates
	b.WriteString("\r\n\x1b[2K")
	b.WriteString(ed.buildCandidateRow(start+perLine, perLine, end, colw))

	// Prompt line: move down, clear line, write prompt + selected candidate preview
	b.WriteString("\r\n\x1b[2K")
	b.WriteString(prompt)
	b.WriteString(string(buf))

	outStr := b.String()

	if keyDebug == "3" {
		esc := make([]byte, 0, len(outStr)*4)
		for _, c := range []byte(outStr) {
			esc = append(esc, []byte(fmt.Sprintf("\\x%02x", c))...)
		}
		fmt.Fprintf(os.Stderr, "TABDEBUG block rawlen=%d escaped=%s\n", len(outStr), string(esc))
	}

	os.Stdout.WriteString(outStr)

	if keyDebug == "2" || keyDebug == "3" {
		fmt.Fprintf(os.Stderr, "TABDEBUG write perLine=%d start=%d end=%d compIndex=%d inCompletion=%v\n",
			perLine, start, end, ed.compIndex, ed.inCompletion)
	}

	// Position cursor on the prompt line at the correct column
	ensureCursor(prompt, buf, cursor)

	ed.inCompletion = true
}

func colWrap(s string, useInverse bool) string {

	col := strings.ToLower(os.Getenv("KUSH_TAB_COLOUR"))
	// map simple names to codes
	m := map[string]string{"black": "30", "red": "31", "green": "32", "yellow": "33", "blue": "34", "magenta": "35", "cyan": "36", "white": "37"}
	if code, ok := m[col]; ok {
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
	if useInverse {
		return "\x1b[7m" + s + "\x1b[0m"
	}
	return s
}

// Prompt reads a single line from stdin with minimal editing capabilities.
func (ed *Editor) Prompt(prompt string) (string, error) {
	// Enter raw mode
	old, err := SetRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	defer RestoreTermios(int(os.Stdin.Fd()), old)

	buf := []rune{}
	cursor := 0
	histIdx := len(ed.history)
	reader := bufio.NewReader(os.Stdin)
	keyDebug := os.Getenv("KUSH_KEYDEBUG") == "1" || os.Getenv("KUSH_KEYDEBUG") == "2" || os.Getenv("KUSH_KEYDEBUG") == "3"
	renderLine(prompt, buf, cursor)

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return "", err
		}
		if keyDebug {
			log.Debugf("KEY read rune=%q code=%d hex=%x", r, r, []byte(string(r)))
		}

		// Ctrl+D
		if r == 4 {
			return "", ErrEOF
		}
		// Ctrl+C: clear current buffer
		if r == 3 {
			buf = []rune{}
			cursor = 0
			ed.resetCompletion()
			renderLine(prompt, buf, cursor)
			continue
		}
		// Ctrl+W: delete previous word
		if r == 23 {
			if cursor > 0 {
				i := cursor - 1
				for i >= 0 && buf[i] == ' ' {
					i--
				}
				for i >= 0 && buf[i] != ' ' {
					i--
				}
				buf = append(buf[:i+1], buf[cursor:]...)
				cursor = i + 1
				renderLine(prompt, buf, cursor)
			}
			continue
		}
		// Ctrl+U: kill to start of line
		if r == 21 {
			if cursor > 0 {
				buf = buf[cursor:]
				cursor = 0
				renderLine(prompt, buf, cursor)
			}
			continue
		}
		// Ctrl+K: kill to end of line
		if r == 11 {
			if cursor < len(buf) {
				buf = buf[:cursor]
				renderLine(prompt, buf, cursor)
			}
			continue
		}
		// newline
		if r == '\n' || r == '\r' {
			line := strings.TrimSpace(string(buf))
			os.Stdout.WriteString("\r\n")
			if line != "" {
				ed.appendHistory(line)
			}
			return line, nil
		}
		// ESC/start of sequences
		if r == 0x1b {
			r1, _, err := reader.ReadRune()
			if err != nil {
				continue
			}
			// CSI sequences
			if r1 == '[' {
				r2, _, err := reader.ReadRune()
				if err != nil {
					continue
				}

				// handle Shift+Tab (ESC [ Z)
				if r2 == 'Z' {
					if ed.compCandidates != nil && len(ed.compCandidates) > 0 {
						if ed.compIndex > 0 {
							ed.compIndex--
						} else {
							ed.compIndex = len(ed.compCandidates) - 1
						}
						_, _, visible, _, _ := ed.compLayout()
						if ed.compIndex < ed.compPageStart {
							ed.compPageStart = ed.compIndex
						} else if ed.compIndex >= ed.compPageStart+visible {
							ed.compPageStart = ed.compIndex - (ed.compIndex % visible)
						}
						ed.renderCandidates(prompt, buf, cursor)
					}
					continue
				}

				// support delete sequence ESC [ 3 ~ and macOS variants like ESC [ 3;3 ~ or ESC [ 3;3
				if r2 == '3' {
					// collect subsequent parameter runes until we hit '~' or a non-digit/';'
					params := []rune{}
					for {
						r3, _, err := reader.ReadRune()
						if err != nil {
							break
						}
						if r3 == '~' {
							// standard delete
							if cursor < len(buf) {
								i := cursor
								for i < len(buf) && buf[i] == ' ' {
									i++
								}
								for i < len(buf) && buf[i] != ' ' {
									i++
								}
								buf = append(buf[:cursor], buf[i:]...)
							}
							break
						}
						// accept digits and semicolon as params, continue
						if (r3 >= '0' && r3 <= '9') || r3 == ';' {
							params = append(params, r3)
							continue
						}
						// some macOS terminals send ESC [ 3 or ESC [ 3;3 without trailing ~
						// decide whether it's opt+delete (delete-right) or opt+backspace (delete-left)
						paramStr := string(params)
						if strings.Contains(paramStr, ";3") {
							// treat as opt+backspace: delete previous word
							if cursor > 0 {
								i := cursor - 1
								for i >= 0 && buf[i] == ' ' {
									i--
								}
								for i >= 0 && buf[i] != ' ' {
									i--
								}
								buf = append(buf[:i+1], buf[cursor:]...)
								cursor = i + 1
							}
						} else {
							// default to delete-right
							if cursor < len(buf) {
								i := cursor
								for i < len(buf) && buf[i] == ' ' {
									i++
								}
								for i < len(buf) && buf[i] != ' ' {
									i++
								}
								buf = append(buf[:cursor], buf[i:]...)
							}
						}
						break
					}
					renderLine(prompt, buf, cursor)
					ensureCursor(prompt, buf, cursor)
					continue
				}
				switch r2 {
				case 'D': // left
					if cursor > 0 {
						cursor--
					}
				case 'C': // right
					if cursor < len(buf) {
						cursor++
					}
				case 'A': // up
					if histIdx > 0 {
						histIdx--
						buf = []rune(ed.history[histIdx])
						cursor = len(buf)
					}
				case 'B': // down
					if histIdx < len(ed.history)-1 {
						histIdx++
						buf = []rune(ed.history[histIdx])
						cursor = len(buf)
					} else if histIdx == len(ed.history)-1 {
						histIdx = len(ed.history)
						buf = []rune{}
						cursor = 0
					}
				case 'H': // home
					cursor = 0
				case 'F': // end
					cursor = len(buf)
				default:
					// ignore
				}
				renderLine(prompt, buf, cursor)
				ensureCursor(prompt, buf, cursor)
				continue
			} else if r1 == 'O' {
				// some terminals send ESC O H/F for home/end
				r2, _, err := reader.ReadRune()
				if err == nil {
					if r2 == 'H' {
						cursor = 0
					} else if r2 == 'F' {
						cursor = len(buf)
					}
				}
				renderLine(prompt, buf, cursor)
				ensureCursor(prompt, buf, cursor)
				continue
			} else {
				// Alt/meta + key (single-rune after ESC)
				switch r1 {
				case 'b': // alt+left
					if cursor > 0 {
						i := cursor - 1
						for i >= 0 && buf[i] == ' ' {
							i--
						}
						for i >= 0 && buf[i] != ' ' {
							i--
						}
						cursor = i + 1
					}
				case 'f': // alt+right
					if cursor < len(buf) {
						i := cursor
						for i < len(buf) && buf[i] == ' ' {
							i++
						}
						for i < len(buf) && buf[i] != ' ' {
							i++
						}
						cursor = i
					}
				case 'd': // alt+delete
					if cursor < len(buf) {
						i := cursor
						for i < len(buf) && buf[i] == ' ' {
							i++
						}
						for i < len(buf) && buf[i] != ' ' {
							i++
						}
						buf = append(buf[:cursor], buf[i:]...)
					}
				case 127: // alt+backspace (ESC+DEL)
					if cursor > 0 {
						i := cursor - 1
						for i >= 0 && buf[i] == ' ' {
							i--
						}
						for i >= 0 && buf[i] != ' ' {
							i--
						}
						buf = append(buf[:i+1], buf[cursor:]...)
						cursor = i + 1
					}
				default:
					// Insert printable alt combos as literal characters.
					// This covers international keyboards where Option+key
					// produces characters like \ { } [ ] ~ on Nordic layouts.
					if r1 >= 32 && r1 != 127 {
						if cursor == len(buf) {
							buf = append(buf, r1)
						} else {
							buf = append(buf[:cursor+1], buf[cursor:]...)
							buf[cursor] = r1
						}
						cursor++
					}
				}
				renderLine(prompt, buf, cursor)
				ensureCursor(prompt, buf, cursor)
				continue
			}
		}

		// Ctrl+H: history viewer
		if r == 8 {
			ed.resetCompletion()
			selected := ed.historyViewer(reader)
			// History may have been modified by deletions — reset index
			histIdx = len(ed.history)
			// Re-query terminal size since we used alt screen
			renderLine(prompt, buf, cursor)
			if selected != "" {
				buf = []rune(selected)
				cursor = len(buf)
				renderLine(prompt, buf, cursor)
			}
			ensureCursor(prompt, buf, cursor)
			continue
		}

		// backspace/delete (direct)
		if r == 127 {
			if cursor > 0 {
				buf = append(buf[:cursor-1], buf[cursor:]...)
				cursor--
				renderLine(prompt, buf, cursor)
				ensureCursor(prompt, buf, cursor)
			}
			continue
		}

		// Tab completion
		if r == 9 {
			// Bash/zsh-style tab completion:
			// 1. Compute candidates from current buffer.
			// 2. If one candidate: insert it (+ space if file, no space if dir).
			// 3. If multiple: insert longest common prefix, show candidates.
			// 4. Repeated Tab with same candidates: cycle through them.
			start, cands := completion.Complete(string(buf), cursor)
			if len(cands) == 0 {
				continue
			}

			// Repeated Tab on same candidate set — cycle.
			if ed.compCandidates != nil && ed.compStart == start && len(ed.compCandidates) > 0 {
				ed.compIndex = (ed.compIndex + 1) % len(ed.compCandidates)
				_, _, visible, _, _ := ed.compLayout()
				if ed.compIndex < ed.compPageStart {
					ed.compPageStart = ed.compIndex
				} else if ed.compIndex >= ed.compPageStart+visible {
					ed.compPageStart = ed.compIndex - (ed.compIndex % visible)
				}
				cand := ed.compCandidates[ed.compIndex]
				newBuf := []rune(cand)
				newLine := append([]rune(string(buf[:start])), newBuf...)
				if cursor < len(buf) {
					newLine = append(newLine, buf[cursor:]...)
				}
				buf = newLine
				cursor = start + len(newBuf)
				ed.renderCandidates(prompt, buf, cursor)
				continue
			}

			if len(cands) == 1 {
				cand := cands[0]
				suffix := " "
				if strings.HasSuffix(cand, "/") {
					suffix = ""
				}
				newBuf := []rune(cand + suffix)
				newLine := append([]rune(string(buf[:start])), newBuf...)
				if cursor < len(buf) {
					newLine = append(newLine, buf[cursor:]...)
				}
				buf = newLine
				cursor = start + len(newBuf)
				ed.resetCompletion()
				renderLine(prompt, buf, cursor)
				ensureCursor(prompt, buf, cursor)
				continue
			}

			// Multiple candidates: insert longest common prefix.
			lcp := longestCommonPrefix(cands)
			currentToken := string(buf[start:cursor])
			if lcp != currentToken {
				// We extended the input — insert the common prefix and
				// re-complete (the narrowed set may be just one).
				newBuf := []rune(lcp)
				newLine := append([]rune(string(buf[:start])), newBuf...)
				if cursor < len(buf) {
					newLine = append(newLine, buf[cursor:]...)
				}
				buf = newLine
				cursor = start + len(newBuf)
				ed.resetCompletion()
				renderLine(prompt, buf, cursor)
				ensureCursor(prompt, buf, cursor)
				continue
			}

			// Common prefix == current token, can't narrow further.
			// Show candidate list and allow cycling.
			ed.compCandidates = cands
			ed.compStart = start
			ed.compIndex = 0
			ed.compPageStart = 0
			ed.renderCandidates(prompt, buf, cursor)
			continue
		}

		// printable runes (>= space)
		if r >= 32 {
			// any normal keypress resets completion state
			ed.resetCompletion()
			if cursor == len(buf) {
				buf = append(buf, r)
			} else {
				buf = append(buf[:cursor+1], buf[cursor:]...)
				buf[cursor] = r
			}
			cursor++
			renderLine(prompt, buf, cursor)
			ensureCursor(prompt, buf, cursor)
			continue
		}
		// ignore others
	}
}
