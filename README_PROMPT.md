KUSH prompt configuration

Kush uses KUSH_PROMPT as the canonical prompt configuration. If KUSH_PROMPT is
unset, the provider falls back to PROMPT / PROMPT_CMD for compatibility.

KUSH_PROMPT supports a minimal token language:
- %% → literal %
- %T → full datetime (YYYY-MM-DD HH:MM:SS)
- %t → time (HH:MM:SS)
- %H → hostname
- %h → hour (HH)
- %m → minute (MM)
- %s → second (SS)
- %p → last path component (basename of PWD)
- %P → full path (PWD)

Dynamic tokens:
- {path/to/script} → run a local script or executable and substitute its stdout (trimmed). The script is executed via sh -c for now; planned: embedded Tengo runner.
- [command] → run external command via sh -c and substitute stdout. For safety, external commands are disabled by default. Enable with KUSH_PROMPT_ALLOW_EXTERNAL=1.

Escaping:
- Use backslash to escape special characters: \% \[ \{ \\

Timeouts and caching:
- PROMPT_TIMEOUT_MS controls per-prompt timeout (milliseconds).
- PROMPT_TTL controls caching (e.g., avoid re-running expensive tokens every prompt).

Security:
- External commands can execute arbitrary code; keep KUSH_PROMPT_ALLOW_EXTERNAL disabled unless you trust the prompt string source.
- Internal script support via Tengo is planned for safer, embeddable prompt scripts.

Examples:
- KUSH_PROMPT="%p %t $ "
- KUSH_PROMPT="{~/.kush/scripts/git_branch} %t $ "
- Enable external commands: export KUSH_PROMPT_ALLOW_EXTERNAL=1
