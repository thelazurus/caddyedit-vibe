# caddyedit

A terminal editor for Caddyfiles: syntax highlighting, a jump-to outline of
site/snippet blocks, a "new site" template wizard, and validate/reload
against a *real* running Caddy — a Docker container by default, a local
`caddy` binary, or (if neither is reachable) Caddy's own tokenizer as a
syntax-only fallback.

## Build

```sh
go build -o caddyedit ./cmd/caddyedit
```

Requires Go 1.24+.

## Run

```sh
./caddyedit -file /path/to/Caddyfile
```

On startup it tries, in order:

1. **Docker** — talks directly to the Docker socket (`/var/run/docker.sock`
   by default) and looks for a single running container whose name or image
   contains "caddy". If found, `validate`/`reload` run via `docker exec`
   inside that container.
2. **Local binary** — if Docker isn't reachable or no container is found,
   falls back to a `caddy` binary on `$PATH`.
3. **Bundled parser** — if neither is available, falls back to Caddy's own
   Caddyfile tokenizer for a structural syntax check only (no adapter/
   directive validation, no reload).

### Flags

| Flag | Default | Purpose |
|---|---|---|
| `-file` | `Caddyfile` | Path to the Caddyfile to edit |
| `-socket` | `/var/run/docker.sock` | Docker socket path |
| `-container` | *(auto-detect)* | Name or ID of the Caddy container |
| `-container-path` | `/etc/caddy/Caddyfile` | Path to the Caddyfile **inside** the container (must match your bind mount) |
| `-no-docker` | `false` | Skip Docker detection entirely |

If auto-detection finds more than one "caddy"-ish container, it lists them
on stderr and asks you to pass `-container` explicitly.

## Keybindings

| Key | Action |
|---|---|
| `Ctrl+S` | Save (writes a `.bak` of the previous on-disk version first) |
| `Ctrl+O` | Toggle the outline panel (jump to a site/snippet block) |
| `Ctrl+N` | New-site wizard (insert a template block at the end of the file) |
| `Ctrl+V` | Save, then validate against the resolved target |
| `Ctrl+R` | Reload the running Caddy instance |
| `q` | Quit (prompts if there are unsaved changes) |
| `Q` | Quit, discarding unsaved changes |
| `?` | Help screen |
| arrows / Home / End / Ctrl+Home / Ctrl+End / PgUp / PgDn | Navigate |
| Enter / Backspace / Delete / Tab | Edit (Enter auto-indents; Tab inserts 4 spaces) |

## New-site templates

The wizard ships templates matching the SSO/reverse-proxy conventions
already in use: plain `reverse_proxy`, `sso-core`, `sso-core-basic`,
`sso-bypass`, `sso-bypass-basic`, and a blank skeleton — each paired with a
Cloudflare-DNS `tls` block. Edit `internal/templates/templates.go` to add or
change templates for your own setup.

## Layout

```
cmd/caddyedit/        CLI entrypoint and flags
internal/buffer/       line-oriented text buffer (cursor movement, edits)
internal/highlight/     Caddyfile token → color mapping
internal/outline/       top-level block extraction for the outline panel
internal/templates/     new-site block skeletons
internal/dockerx/       minimal Docker Engine API client over the socket
internal/validate/      validate/reload orchestration (docker/binary/parser)
internal/app/           Bubble Tea model and view
```

## Known limitations (v1)

- No horizontal scrolling — very long lines are truncated in the editor
  view (the underlying file content is untouched).
- The outline/highlighter are heuristic, not a full Caddyfile grammar —
  good enough for readability and navigation, not a replacement for
  `caddy validate`.
- Only one level of undo isn't implemented at all yet — the `.bak` file
  written on save is your safety net.
