# kush — a tiny custom shell line editor

Kush is a small terminal-focused shell/line editor project. It provides a custom
single-line editor (no readline/go-prompt dependency), builtins, command
history, and a future PTY-backed runner for interactive programs.

This README documents a few terminal quirks and how to configure macOS
terminals so Option/Alt behaves as Meta (Esc+), which kush expects for
Alt+arrow / Option+Backspace / Option+Delete behaviours.

## Key behaviour

- Movement: Left/Right/Up/Down, Home/End are supported (CSI and OSC variants).
- Meta/Option: The editor recognizes ESC-prefixed sequences as Alt/Meta (e.g.
  ESC b / ESC f for alt-left/alt-right).
- Option/Alt+Backspace / Option/Alt+Delete: Not all terminals send distinct
  sequences by default — see configuration below.
- Ctrl shortcuts: Ctrl+C clears the current line, Ctrl+D signals EOF,
  and common kills are supported: Ctrl+W (backward-word), Ctrl+U (kill to
  start of line), Ctrl+K (kill to end of line).

## macOS / iTerm2 / Terminal.app configuration

Kush expects Option (Alt) to act as Meta — i.e. to send an Escape prefix
before the modified key. If Option is left as "normal" in some terminals,
Option+Backspace may send the same byte as Backspace and cannot be distinguished
by a terminal program.

iTerm2 (recommended)

1. Preferences → Profiles → Keys
2. Set Left Option (and/or Right Option) to: `Esc+`

This makes Option+Backspace send `ESC 0x7f` which kush treats as Alt+Backspace
and will delete the previous word.

Terminal.app (macOS)

1. Preferences → Profiles → Keyboard
2. Click `+` to add a mapping
3. For the Key, press Option+Backspace
4. Set "Action" to "Send string to shell:"
5. Paste the value: `\033\177` (that's ESC followed by DEL / 0x7f)

Other terminals

- Alacritty: set `alt_send_escape: true` in your config, or add a key mapping
  for `Option+Backspace` to send `\x1b\x7f`.
- GNOME Terminal / VTE-based: check the profile keyboard preferences and
  enable "Meta sends Escape" or add a custom keybinding.

Troubleshooting

- To inspect what your terminal actually sends for a key, run:

  cat -v

  Then press the key. Output examples:

  - `^?` indicates DEL (0x7f)
  - `^H` indicates Backspace (0x08)
  - `^[` indicates ESC and will show sequences like `^[[3~` for Delete

- If Option+Backspace prints the same as Backspace (e.g. both produce `^?`),
  your terminal is not sending an ESC prefix. Configure Option-as-Esc as
  above or use the Ctrl+W shortcut (supported by kush) as a cross-terminal
  workaround.

Support and next steps

- The editor normalises a variety of CSI sequences (e.g. `ESC [ 3 ~`,
  `ESC [ 3;3 ~`, `ESC O H`) so macOS and Linux variants work.
- If you run into other terminal-specific issues, enable the key debug mode
  to inspect raw runes: run with `KUSH_KEYDEBUG=1 ./kush` and paste the
  stderr output.

Debugging and auto-reload

- KUSH_DEBUG controls informational diagnostics:
  - `0` (default/unset): quiet — only real errors printed.
  - `1`: verbose — prints messages such as "kush: aliases: loaded ..." and
    alias warnings when setting aliases.

- Auto-reload on SIGHUP: kush listens for SIGHUP and will reload aliases into
  the in-memory cache when it receives the signal. This mirrors the
  `alias -r` command and is useful for external workflows that update the
  aliases file and then signal running shells to pick up changes.

Examples

Reload aliases from the running shell:

- `alias -r` — reload aliases from the canonical file into kush's cache.

Reload aliases from outside the shell:

- `kill -HUP <pid-of-kush>` — send SIGHUP to the running kush process; if
  `KUSH_DEBUG=1` you'll see a message on stderr confirming reload.

Contributing

PRs welcome. Keep the editor minimal and rune-aware; prefer configuration
notes rather than trying to invent terminal-side heuristics when terminals
can be configured to send canonical sequences.
