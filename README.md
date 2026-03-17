# kush — a tiny custom shell

Kush is a minimal terminal shell written in Go. It provides a custom
single-line editor (no readline or go-prompt dependency), command aliases,
persistent history, and a PTY-backed command runner so interactive programs
behave correctly.

## Features

- **Line editor** — single-line editing with cursor movement, history
  navigation, and word-level operations. No external TUI libraries.
- **PTY runner** — commands run inside a pseudoterminal (`openpty` on macOS,
  `posix_openpt` on Linux) with automatic fallback to plain `exec` on
  unsupported platforms.
- **Aliases** — loaded from `~/.kush_aliases` (or `$KUSH_ALIASES`), with
  two-pass chained expansion and live reload via `alias -r` or SIGHUP.
- **Builtins** — `cd`, `history`, `alias`, `unalias`, `reload`, `which`,
  `checksum` (stub).
- **Config** — optional `~/.kush_config` for key=value settings like
  `PATH_FIRST`.

## Building

```sh
make          # produces ./kush
make clean    # removes binaries
```

Requires Go 1.25+ and cgo on macOS (for `openpty`).

## Key bindings

| Key               | Action                       |
|--------------------|------------------------------|
| Left / Right       | Move cursor                  |
| Up / Down          | Navigate history             |
| Home / End         | Jump to start / end of line  |
| Alt+Left / Alt+Right | Move by word              |
| Backspace          | Delete character left        |
| Alt+Backspace      | Delete word left             |
| Delete             | Delete word right            |
| Ctrl+W             | Kill word left               |
| Ctrl+U             | Kill to start of line        |
| Ctrl+K             | Kill to end of line          |
| Ctrl+C             | Clear current line           |
| Ctrl+D             | Exit (EOF)                   |

## Terminal configuration

Kush expects Option/Alt to send an Escape prefix (Meta mode). If word-delete
shortcuts don't work, configure your terminal:

**iTerm2:** Preferences → Profiles → Keys → set Left Option to `Esc+`.

**Terminal.app:** Preferences → Profiles → Keyboard → add a mapping for
Option+Backspace that sends `\033\177`.

**Alacritty:** set `alt_send_escape: true` or add a key mapping for
Option+Backspace → `\x1b\x7f`.

To inspect what your terminal sends for a key, run `cat -v` and press it.

## Debug mode

Set `KUSH_DEBUG` to control diagnostic output on stderr:

- `0` (default) — quiet; only real errors are printed.
- `1` — verbose; alias loading, reload events, and runner diagnostics.
- `2` — trace; detailed PTY lifecycle, termios state, and goroutine events.

Key debug mode: run with `KUSH_KEYDEBUG=1` to log raw key codes to stderr.

## Aliases

Aliases are loaded from `~/.kush_aliases` (override with `$KUSH_ALIASES`).
Supported formats:

```
alias ll='ls -la'
ll='ls -la'
ll=ls -la
```

Chained aliases work: if `la` expands to `ls -la` and `ls` expands to
`ls --color=yes`, the result is `ls --color=yes -la` (without duplicate flags).

Reload from within the shell with `alias -r` or from outside with
`kill -HUP <pid>`.

## Project structure

```
main.go                          Entry point
internal/
  shell/shell.go                 REPL loop
  ed/                            Line editor and termios helpers
    lineeditor.go                Editor implementation
    term_darwin.go               macOS termios (raw mode)
    term_linux.go                Linux termios
  runner/                        Command execution
    pty_runner.go                PTY runner + plain-exec fallback
    pty_darwin.go                openpty via cgo (macOS)
    pty_linux.go                 posix_openpt (Linux)
    pty_unsupported.go           Stub for other platforms
  aliases/aliases.go             Alias loading, expansion, persistence
  builtins/builtins.go           Shell builtins
  config/config.go               Config file loader
  log/log.go                     Levelled logging
```
