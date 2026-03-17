package ed

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrEOF is returned when the user signals end-of-input (e.g. Ctrl+D).
var ErrEOF = errors.New("eof")

// Editor provides a minimal ANSI/raw-mode line editor that doesn't use a
// fullscreen library. It edits a single line, supports left/right, backspace,
// simple history and tab as literal.
type Editor struct {
	history  []string
	histPath string
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
	return &Editor{history: history, histPath: hist}, nil
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

// renderLine redraws the prompt and buffer, positioning cursor.
func renderLine(prompt string, buf []rune, cursor int) {
	// carriage return, clear line, write prompt+buffer, move cursor
	os.Stdout.WriteString("\r\x1b[K")
	os.Stdout.WriteString(prompt)
	os.Stdout.WriteString(string(buf))
	// move cursor to position after prompt+cursor
	pos := len(prompt) + cursor
	// move to column pos: write \r then forward
	os.Stdout.WriteString("\r")
	if pos > 0 {
		os.Stdout.WriteString("\x1b[" + fmt.Sprintf("%d", pos) + "C")
	}
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
	keyDebug := os.Getenv("KUSH_KEYDEBUG") == "1"
	renderLine(prompt, buf, cursor)

	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			return "", err
		}
		if keyDebug {
			fmt.Fprintf(os.Stderr, "KEY read rune=%q code=%d hex=%x\n", r, r, []byte(string(r)))
		}

		// Ctrl+D
		if r == 4 {
			return "", ErrEOF
		}
		// Ctrl+C: clear current buffer
		if r == 3 {
			buf = []rune{}
			cursor = 0
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
				case 127, 8: // alt+backspace (ESC+DEL or ESC+BS)
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
					// ignore other alt combos
				}
				renderLine(prompt, buf, cursor)
				continue
			}
		}

		// backspace/delete (direct)
		if r == 127 || r == 8 {
			if cursor > 0 {
				buf = append(buf[:cursor-1], buf[cursor:]...)
				cursor--
				renderLine(prompt, buf, cursor)
			}
			continue
		}

		// printable runes (>= space)
		if r >= 32 {
			if cursor == len(buf) {
				buf = append(buf, r)
			} else {
				buf = append(buf[:cursor+1], buf[cursor:]...)
				buf[cursor] = r
			}
			cursor++
			renderLine(prompt, buf, cursor)
			continue
		}
		// ignore others
	}
}
