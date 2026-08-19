// Package validate checks Caddyfile content, preferring the real `caddy`
// binary (inside a Docker container if that's where Caddy runs, or on the
// local PATH) and falling back to Caddy's own tokenizer for a syntax-only
// check when neither is reachable.
package validate

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/thelazurus/caddyedit-vibe/internal/dockerx"
)

type TargetKind int

const (
	TargetNone TargetKind = iota
	TargetDocker
	TargetLocalBinary
)

// Target describes where `caddy` commands should actually run.
type Target struct {
	Kind          TargetKind
	Docker        *dockerx.Client
	ContainerID   string
	ContainerName string
	ContainerPath string // path to the Caddyfile inside the container
	LocalBinary   string // resolved path from exec.LookPath, e.g. "/usr/bin/caddy"
}

func (t Target) Description() string {
	switch t.Kind {
	case TargetDocker:
		return fmt.Sprintf("docker:%s", t.ContainerName)
	case TargetLocalBinary:
		return "local caddy binary"
	default:
		return "bundled parser (no caddy binary reachable)"
	}
}

type Result struct {
	OK      bool
	Source  string
	Message string
}

// Validate checks the on-disk file at hostPath (content is what was just
// written there) for syntax/adapter errors.
func Validate(ctx context.Context, hostPath string, content []byte, t Target) Result {
	switch t.Kind {
	case TargetDocker:
		res, err := t.Docker.Exec(ctx, t.ContainerID, []string{"caddy", "validate", "--config", t.ContainerPath, "--adapter", "caddyfile"})
		if err != nil {
			return Result{OK: false, Source: t.Description(), Message: "docker exec failed: " + err.Error()}
		}
		return Result{OK: res.ExitCode == 0, Source: t.Description(), Message: combinedOutput(res.Stdout, res.Stderr)}

	case TargetLocalBinary:
		cmd := exec.CommandContext(ctx, t.LocalBinary, "validate", "--config", hostPath, "--adapter", "caddyfile")
		out, err := cmd.CombinedOutput()
		ok := err == nil
		return Result{OK: ok, Source: t.Description(), Message: strings.TrimSpace(string(out))}

	default:
		_, err := caddyfile.Parse(hostPath, content)
		if err != nil {
			return Result{OK: false, Source: t.Description(), Message: err.Error()}
		}
		return Result{OK: true, Source: t.Description(), Message: "syntax OK (structure only — no adapter/directive validation)"}
	}
}

// Reload asks the running Caddy instance to reload its config. Only
// possible with a Docker or local binary target.
func Reload(ctx context.Context, t Target) Result {
	switch t.Kind {
	case TargetDocker:
		res, err := t.Docker.Exec(ctx, t.ContainerID, []string{"caddy", "reload", "--config", t.ContainerPath, "--adapter", "caddyfile"})
		if err != nil {
			return Result{OK: false, Source: t.Description(), Message: "docker exec failed: " + err.Error()}
		}
		return Result{OK: res.ExitCode == 0, Source: t.Description(), Message: combinedOutput(res.Stdout, res.Stderr)}

	case TargetLocalBinary:
		cmd := exec.CommandContext(ctx, t.LocalBinary, "reload", "--config", "-", "--adapter", "caddyfile")
		out, err := cmd.CombinedOutput()
		return Result{OK: err == nil, Source: t.Description(), Message: strings.TrimSpace(string(out))}

	default:
		return Result{OK: false, Source: t.Description(), Message: "no caddy binary or container reachable — reload manually"}
	}
}

func combinedOutput(stdout, stderr string) string {
	s := strings.TrimSpace(stdout + stderr)
	if s == "" {
		s = "(no output)"
	}
	return s
}

// ResolveLocalBinary returns the resolved path to a `caddy` binary on PATH,
// or "" if not found.
func ResolveLocalBinary() string {
	p, err := exec.LookPath("caddy")
	if err != nil {
		return ""
	}
	return p
}
