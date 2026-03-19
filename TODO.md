# kush — TODO

## Done

- [x] Persistent history (~/.kush_history)
- [x] PTY-backed runner (openpty on darwin/linux, plain exec fallback)
- [x] Aliases (~/.kush_aliases, KUSH_ALIASES env override, SIGHUP reload)
- [x] Ctrl+W/U/K kill shortcuts
- [x] Alt/Option word movement and word-delete
- [x] Config loader (~/.kush_config with PATH_FIRST)
- [x] Tab completion (path/command candidates, cycling)
- [x] Export builtin (quoted values, prompt invalidation)
- [x] SIGWINCH propagation to PTY
- [x] Ctrl+H history viewer (ANSI overlay, colour, search, delete)

## Phase 1 — Tengo & HTTP

### Tengo runtime
- [x] Use `github.com/d5/tengo` runtime (note: upstream v3 tag uses v2 module path; project currently pins `github.com/d5/tengo/v2 v2.17.0` to avoid module-path mismatch)
- [x] `run <script.tengo>` builtin — execute a Tengo script with args
- [x] `eval '<expr>'` builtin — inline one-liner execution
- [x] Blessed script path: `$KUSH_SCRIPTS` (default `~/.kush/scripts/`) — auto-registration of `.tengo` files
- [x] Auto-register `.tengo` files in blessed path as shell builtins
- [x] Preloaded `kush` module: env get/set, args, cwd, exit, prompt invalidation
- [ ] `fmt` module: cfmt-based print/printf/println, colour helpers (red, cyan, bold, etc.)
- [ ] `fs` module: read, write, append, ls, stat, mkdir, exists, glob
- [ ] `json` module: encode, decode, pretty
- [ ] `hash` module: md5, sha1, sha256, sha512, crc32 (string or file path)
- [ ] `cli` module: climate/arg wrapper for flag/arg parsing in scripts
- [ ] `exec` module: run shell commands, capture stdout/stderr, exit code
- [x] `http` module: expose the native HTTP functions to scripts

### Native HTTP builtins
- [x] `get <url>` — HTTP GET, stdout output, pipe-friendly, `-H` headers, `-j` JSON parse
- [x] `post <url> <data>` — HTTP POST, auto content-type detection
- [x] `put <url> <data>` — HTTP PUT
- [x] `delete <url>` — HTTP DELETE
- [x] `head <url>` — HTTP HEAD, show response headers
- [x] `fetch <url> [-o file]` — download to file (wget/curl equivalent)
- [x] Internal HTTP client shared between builtins and Tengo `http` module

### Other native builtins
- [ ] `checksum <algo> <file...>` — md5/sha1/sha256/sha512/crc32
- [ ] `whois <domain>` — native whois lookup
- [ ] `dig <domain> [type]` — DNS lookup (A, AAAA, MX, NS, TXT, etc.)

## Phase 2 — Infrastructure & OCI

### OCI / container management
- [ ] `oci` command: unified interface, auto-detects Docker/Podman/macOS tooling
- [ ] Docker convenience wrappers (ps, images, run, stop, rm, logs, inspect, pull)
- [ ] Podman support as alternative backend
- [ ] Direct Docker API for diagnostics and start/stop (unix socket)

### Kubernetes
- [ ] K8s convenience wrappers around kubectl (pods, services, deployments, logs)
- [ ] Direct K8s API for diagnostics (kubeconfig-based)

### Network modules
- [ ] `net` Tengo module: tcp_connect, dns_lookup, ping, basic socket ops
- [ ] `ssh` Tengo module: wraps system ssh initially

### Integrate local repos
- [ ] Detect and expose grimdork repos (name, base) as builtins

## Phase 3 — Standalone & Polish

- [ ] Native SSH client (Go crypto/ssh) — kush as drop-in for VMs/containers
- [ ] Spelling/grammar integration (sajari/fuzzy + LanguageTool)
- [ ] Optional macOS native spell bridge (CGO)
- [ ] Script safety model: sandbox mode, dry-run flag
- [ ] Tests and CI
- [ ] Plugin registry and loader
