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
	"time"
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
	pos := len([]rune(visiblePrompt)) + cursor
	os.Stdout.WriteString("\r")
	if pos > 0 {
		os.Stdout.WriteString("\x1b[" + fmt.Sprintf("%d", pos) + "G")
	}
	if os.Getenv("KUSH_KEYDEBUG") == "2" || os.Getenv("KUSH_KEYDEBUG") == "3" {
		fmt.Fprintf(os.Stderr, "CURDEBUG visiblePrompt=%q promptLen=%d cursor=%d pos=%d\n", visiblePrompt, len([]rune(visiblePrompt)), cursor, pos)
	}
}

// renderLine redraws the prompt and buffer, positioning cursor.
func renderLine(prompt string, buf []rune, cursor int) {
	// carriage return, clear line, write prompt+buffer, move cursor
	os.Stdout.WriteString("\r\x1b[K")
	os.Stdout.WriteString(prompt)
	os.Stdout.WriteString(string(buf))
	// move cursor to position after prompt+cursor
	pos := len(prompt) + cursor
	// move to column pos: write \r then forward (use CSI nG for absolute column)
	os.Stdout.WriteString("\r")
	if pos > 0 {
		os.Stdout.WriteString("\x1b[" + fmt.Sprintf("%d", pos) + "G")
	}
}

// renderCandidates prints two lines of candidates below the prompt, highlighting
// the current selection with inverse-video. It attempts simple one-column-per-choice layout.
func (ed *Editor) renderCandidates(prompt string, buf []rune, cursor int) {
	cands := ed.compCandidates
	if len(cands) == 0 {
		return
	}
	// If we're already in completion mode, reuse the displayed candidate rows:
	// only update the prompt preview/highlight without reflowing the block.
	if ed.inCompletion {
		// redraw prompt preview (plain-text) and position cursor
		showPrompt := prompt
		firstPreview := ""
		if len(cands) > 0 {
			firstPreview = cands[ed.compIndex]
		}
		// build preview string so we can optionally dump exact bytes under deep debug
		previewBuf := &bytes.Buffer{}
		previewBuf.WriteString("\r\x1b[K")
		previewBuf.WriteString(showPrompt)
		if firstPreview != "" {
			previewBuf.WriteString(" ")
			previewBuf.WriteString(firstPreview)
		}
		previewStr := previewBuf.String()
		if os.Getenv("KUSH_KEYDEBUG") == "3" {
			// escape bytes for stderr-safe capture
			esc := make([]byte, 0, len(previewStr)*4)
			for _, c := range []byte(previewStr) {
				esc = append(esc, []byte(fmt.Sprintf("\\x%02x", c))...)
			}
			fmt.Fprintf(os.Stderr, "TABDEBUG preview rawlen=%d escaped=%s\n", len(previewStr), string(esc))
		}
		os.Stdout.WriteString(previewStr)
		renderLine(prompt, buf, cursor)
		ensureCursor(prompt, buf, cursor)
		return
	}
	// If session-level disable is set, fall back to a conservative simple render
	// that avoids any scroll-region or DECSC/DECRC manipulation which have
	// exhibited terminal-driver races on some environments.
	if ed.disableDECSTBM {
		// simple conservative render: write a newline, two compact rows, then redraw prompt
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
		maxw := 0
		for _, c := range cands {
			if w := len(c); w > maxw {
				maxw = w
			}
		}
		if maxw == 0 {
			maxw = 1
		}
		colw := maxw + 2
		perLine := cols / colw
		if perLine < 1 {
			perLine = 1
		}
		start := ed.compPageStart
		if start < 0 {
			start = 0
		}
		// ensure we always write exactly two rows worth of slots (pad with clears)
		totalSlots := perLine * 2
		end := start + totalSlots
		if end > len(cands) {
			end = len(cands)
		}
		// build conservative text: explicitly clear and pad lines so every redraw is full
		b := &bytes.Buffer{}
		// We want candidates printed ABOVE the current prompt. Save cursor, move up two
		// lines, write the two candidate rows, then move back to prompt line and
		// print a refreshed prompt with the first candidate shown after the prompt
		// (visual-only; buffer is unchanged).
		b.WriteString("\x1b7") // save
		// move up two lines (if at top this will clamp)
		b.WriteString("\x1b[2A")
		// first candidate row: build full line then write once
		for row := 0; row < 1; row++ {
			lineBuf := &bytes.Buffer{}
			for idx := 0; idx < perLine; idx++ {
				i := start + idx
				if i < end {
					s := cands[i]
					if i == ed.compIndex {
						lineBuf.WriteString(colWrap(s, true))
					} else {
						lineBuf.WriteString(s)
					}
					pad := colw - len(s)
					for p := 0; p < pad; p++ {
						lineBuf.WriteString(" ")
					}
				} else {
					for p := 0; p < colw; p++ {
						lineBuf.WriteString(" ")
					}
				}
			}
			b.WriteString("\x1b[2K\r")
			b.WriteString(lineBuf.String())
		}
		// move to next line and write second row similarly
		b.WriteString("\x1b[1B")
		lineBuf2 := &bytes.Buffer{}
		for idx := 0; idx < perLine; idx++ {
			i := start + perLine + idx
			if i < end {
				s := cands[i]
				if i == ed.compIndex {
					lineBuf2.WriteString(colWrap(s, true))
				} else {
					lineBuf2.WriteString(s)
				}
				pad := colw - len(s)
				for p := 0; p < pad; p++ {
					lineBuf2.WriteString(" ")
				}
			} else {
				for p := 0; p < colw; p++ {
					lineBuf2.WriteString(" ")
				}
			}
		}
		b.WriteString("\x1b[2K\r")
		b.WriteString(lineBuf2.String())
		// clear rest of screen below second row
		b.WriteString("\x1b[0J")
		// now move down to original prompt line
		b.WriteString("\x1b[1B")
		// clear prompt line and write prompt + space + preview (first candidate)
		b.WriteString("\x1b[2K\r")
		// ensure prompt begins with $ if provided; use prompt as-is
		showPrompt := prompt
		firstPreview := ""
		if len(cands) > 0 {
			firstPreview = cands[0]
		}
		b.WriteString(showPrompt)
		if firstPreview != "" {
			b.WriteString(" ")
			b.WriteString(firstPreview) // plain-text preview
		}
		// restore cursor
		b.WriteString("\x1b8")
		// write conservative block atomically
		outStr := b.String()
		// when deep debug requested, dump escaped buffer to stderr for byte-level analysis
		if os.Getenv("KUSH_KEYDEBUG") == "3" {
			// print hex-escaped bytes to stderr (not raw) for clean capture
			esc := make([]byte, 0, len(outStr)*4)
			for _, c := range []byte(outStr) {
				esc = append(esc, []byte(fmt.Sprintf("\\x%02x", c))...)
			}
			fmt.Fprintf(os.Stderr, "TABDEBUG conservative rawlen=%d escaped=%s\n", len(outStr), string(esc))
		}
		os.Stdout.WriteString(outStr)
		// optional short pause for timing-sensitive terminals when deep debug
		if os.Getenv("KUSH_KEYDEBUG") == "3" {
			// 30ms pause
			importTimeSleep30ms()
		}
		// debug buffer length
		if os.Getenv("KUSH_KEYDEBUG") == "2" || os.Getenv("KUSH_KEYDEBUG") == "3" {
			fmt.Fprintf(os.Stderr, "TABDEBUG conservative write len=%d slots=%d perLine=%d start=%d end=%d compIndex=%d\n", b.Len(), totalSlots, perLine, start, end, ed.compIndex)
		}
		// redraw prompt explicitly
		renderLine(prompt, buf, cursor)
		ensureCursor(prompt, buf, cursor)
		// mark completion-mode active so subsequent tabs reuse rows
		ed.inCompletion = true
		return
	}

	// get terminal width via ioctl(TIOCGWINSZ) -> cols, fallback to $COLUMNS then 80
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
	// compute max width of a candidate (limited)
	maxw := 0
	for _, c := range cands {
		if w := len(c); w > maxw {
			maxw = w
		}
	}
	if maxw == 0 {
		maxw = 1
	}
	// one candidate per column (no wrapping of choice text). column width = maxw + 2
	colw := maxw + 2
	perLine := cols / colw
	if perLine < 1 {
		perLine = 1
	}
	// total visible = perLine * 2
	visible := perLine * 2
	// normalize compPageStart to a multiple of visible and clamp
	if visible > 0 {
		ed.compPageStart = (ed.compPageStart / visible) * visible
	}
	if ed.compPageStart < 0 {
		ed.compPageStart = 0
	}
	if ed.compPageStart >= len(cands) {
		ed.compPageStart = 0
	}
	start := ed.compPageStart
	end := start + visible
	if end > len(cands) {
		end = len(cands)
	}
	// verbose debug for geometry when requested
	if os.Getenv("KUSH_KEYDEBUG") == "2" {
		fmt.Fprintf(os.Stderr, "TABDEBUG cols=%d ws.Col=%d maxw=%d colw=%d perLine=%d visible=%d compPageStart=%d start=%d end=%d compIndex=%d\n", cols, ws.Col, maxw, colw, perLine, visible, ed.compPageStart, start, end, ed.compIndex)
	}
	// Decide whether it's safe to print above the prompt. Use ws.Row (terminal
	// height) as a heuristic: require enough rows so the full block can be placed
	// above without hitting top-of-screen clamping. blockHeight is number of rows
	// we need above the prompt (two rows for candidates).
	blockHeight := 2
	canAbove := false
	if ws.Row >= uint16(blockHeight+2) {
		canAbove = true
	}
	// build buffer depending on above/below choice
	bufw := &bytes.Buffer{}
	row1 := []int{}
	row2 := []int{}
	if canAbove {
		// write above: save cursor, move up two lines, draw rows, preview prompt, restore
		bufw.WriteString("\x1b7") // save cursor
		bufw.WriteString("\x1b[2A")
		// first row: build full line and write once
		line1 := &bytes.Buffer{}
		for i := start; i < start+perLine && i < end; i++ {
			row1 = append(row1, i)
			s := cands[i]
			if i == ed.compIndex {
				line1.WriteString(colWrap(s, true))
			} else {
				line1.WriteString(s)
			}
			pad := colw - len(s)
			for p := 0; p < pad; p++ {
				line1.WriteString(" ")
			}
		}
		bufw.WriteString("\x1b[2K\r")
		bufw.WriteString(line1.String())
		// second row
		bufw.WriteString("\x1b[1B")
		line2 := &bytes.Buffer{}
		for i := start + perLine; i < start+2*perLine && i < end; i++ {
			row2 = append(row2, i)
			s := cands[i]
			if i == ed.compIndex {
				line2.WriteString(colWrap(s, true))
			} else {
				line2.WriteString(s)
			}
			pad := colw - len(s)
			for p := 0; p < pad; p++ {
				line2.WriteString(" ")
			}
		}
		bufw.WriteString("\x1b[2K\r")
		bufw.WriteString(line2.String())
		bufw.WriteString("\x1b[0J")
		bufw.WriteString("\x1b[1B")
		bufw.WriteString("\x1b[2K\r")
		showPrompt := prompt
		firstPreview := ""
		if len(cands) > 0 {
			firstPreview = cands[ed.compIndex]
		}
		bufw.WriteString(showPrompt)
		if firstPreview != "" {
			bufw.WriteString(" ")
			bufw.WriteString(firstPreview) // plain-text preview
		}
		bufw.WriteString("\x1b8")
	} else {
		// fallback: downward conservative render (original behaviour)
		bufw.WriteString("\r\n")
		// first down row: build a full line and write
		firstDown := &bytes.Buffer{}
		for idx := 0; idx < perLine; idx++ {
			i := start + idx
			if i < end {
				s := cands[i]
				if i == ed.compIndex {
					firstDown.WriteString(colWrap(s, true))
				} else {
					firstDown.WriteString(s)
				}
				pad := colw - len(s)
				for p := 0; p < pad; p++ {
					firstDown.WriteString(" ")
				}
			} else {
				for p := 0; p < colw; p++ {
					firstDown.WriteString(" ")
				}
			}
		}
		bufw.WriteString("\x1b[2K\r")
		bufw.WriteString(firstDown.String())
		bufw.WriteString("\r\n")
		secondDown := &bytes.Buffer{}
		for idx := 0; idx < perLine; idx++ {
			i := start + perLine + idx
			if i < end {
				s := cands[i]
				if i == ed.compIndex {
					secondDown.WriteString(colWrap(s, true))
				} else {
					secondDown.WriteString(s)
				}
				pad := colw - len(s)
				for p := 0; p < pad; p++ {
					secondDown.WriteString(" ")
				}
			} else {
				for p := 0; p < colw; p++ {
					secondDown.WriteString(" ")
				}
			}
		}
		bufw.WriteString("\x1b[2K\r")
		bufw.WriteString(secondDown.String())
		bufw.WriteString("\x1b[0J")
		bufw.WriteString("\x1b[2K\r")
		showPrompt := prompt
		firstPreview := ""
		if len(cands) > 0 {
			firstPreview = cands[ed.compIndex]
		}
		bufw.WriteString(showPrompt)
		if firstPreview != "" {
			bufw.WriteString(" ")
			bufw.WriteString(firstPreview) // plain-text preview
		}
	}
	// write everything in one shot
	outStr := bufw.String()
	// when deep debug requested, dump escaped buffer to stderr for byte-level analysis
	if os.Getenv("KUSH_KEYDEBUG") == "3" {
		esc := make([]byte, 0, len(outStr)*4)
		for _, c := range []byte(outStr) {
			esc = append(esc, []byte(fmt.Sprintf("\\x%02x", c))...)
		}
		fmt.Fprintf(os.Stderr, "TABDEBUG block rawlen=%d escaped=%s\n", len(outStr), string(esc))
	}
	os.Stdout.WriteString(outStr)
	// optional short pause for timing-sensitive terminals when deep debug
	if os.Getenv("KUSH_KEYDEBUG") == "3" {
		importTimeSleep30ms()
	}
	// debug rows to stderr
	if os.Getenv("KUSH_KEYDEBUG") == "2" {
		fmt.Fprintf(os.Stderr, "TABDEBUG rows row1=%v row2=%v\n", row1, row2)
	}
	// finally ensure cursor is positioned inside the prompt at len(prompt)+cursor
	ensureCursor(prompt, buf, cursor)
	os.Stdout.Sync()
	// mark completion-mode active so subsequent tabs reuse rows
	ed.inCompletion = true
}

// colWrap wraps s in the configured tab colour using ANSI; if useInverse true, prefer colour then inverse fallback.
func importTimeSleep30ms() {
	// small helper to avoid importing time in multiple spots; we call this only
	// when deep debug is enabled to probe timing-sensitive races.
	// Note: tiny sleep is optional and gated on KUSH_KEYDEBUG==3.
	// (kept small to minimize test interference)
	time.Sleep(30 * time.Millisecond)
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
						// page so index is visible
						// compute perLine same way as renderCandidates
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
						maxw := 0
						for _, c := range ed.compCandidates {
							if l := len(c); l > maxw {
								maxw = l
							}
						}
						if maxw == 0 {
							maxw = 1
						}
						colw := maxw + 2
						perLine := cols / colw
						if perLine < 1 {
							perLine = 1
						}
						visible := perLine * 2
						// adjust page start so compIndex lies within [pageStart, pageStart+visible)
						if ed.compIndex < ed.compPageStart {
							ed.compPageStart = ed.compIndex
						} else if ed.compIndex >= ed.compPageStart+visible {
							ed.compPageStart = ed.compIndex - (ed.compIndex % visible)
						}
						// redraw candidates
						ed.renderCandidates(prompt, buf, cursor)
						renderLine(prompt, buf, cursor)
						ensureCursor(prompt, buf, cursor)
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
				ensureCursor(prompt, buf, cursor)
				continue
			}
		}

		// backspace/delete (direct)
		if r == 127 || r == 8 {
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
			// call completer
			start, cands := completion.Complete(string(buf), cursor)
			if len(cands) == 0 {
				// nothing
				continue
			}
			// If we have an existing candidate list and same start, cycle
			if ed.compCandidates != nil && ed.compStart == start && len(ed.compCandidates) > 0 {
				ed.compIndex = (ed.compIndex + 1) % len(ed.compCandidates)
				// ensure page contains index
				// compute perLine similar to renderCandidates
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
				maxw := 0
				for _, c := range ed.compCandidates {
					if l := len(c); l > maxw {
						maxw = l
					}
				}
				if maxw == 0 {
					maxw = 1
				}
				colw := maxw + 2
				perLine := cols / colw
				if perLine < 1 {
					perLine = 1
				}
				visible := perLine * 2
				if ed.compIndex < ed.compPageStart {
					ed.compPageStart = ed.compIndex
				} else if ed.compIndex >= ed.compPageStart+visible {
					ed.compPageStart = ed.compIndex - (ed.compIndex % visible)
				}
				cand := ed.compCandidates[ed.compIndex]
				// replace buffer from start..cursor with cand
				newBuf := []rune(cand)
				newLine := append([]rune(string(buf[:start])), newBuf...)
				// append rest of original after cursor
				if cursor < len(buf) {
					newLine = append(newLine, buf[cursor:]...)
				}
				buf = newLine
				cursor = start + len(newBuf)
				renderLine(prompt, buf, cursor)
				// deterministic reposition and debug
				ensureCursor(prompt, buf, cursor)
				// redraw candidate page below
				ed.renderCandidates(prompt, buf, cursor)
				continue
			}
			// fresh candidate list
			ed.compCandidates = cands
			ed.compStart = start
			ed.compIndex = 0
			ed.compPageStart = 0
			if len(cands) == 1 {
				// single candidate -> insert with trailing space if appropriate
				cand := cands[0]
				newBuf := []rune(cand + " ")
				newLine := append([]rune(string(buf[:start])), newBuf...)
				if cursor < len(buf) {
					newLine = append(newLine, buf[cursor:]...)
				}
				buf = newLine
				cursor = start + len(newBuf)
				renderLine(prompt, buf, cursor)
				ensureCursor(prompt, buf, cursor)
				continue
			}
			// multiple candidates: show two-line page and leave buffer unchanged
			ed.renderCandidates(prompt, buf, cursor)
			renderLine(prompt, buf, cursor)
			ensureCursor(prompt, buf, cursor)
			continue
		}

		// printable runes (>= space)
		if r >= 32 {
			// any normal keypress resets completion state
			ed.compCandidates = nil
			ed.compIndex = 0
			ed.compPageStart = 0
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
