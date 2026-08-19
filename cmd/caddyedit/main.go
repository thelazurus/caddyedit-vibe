// Command caddyedit is a terminal editor for Caddyfiles: syntax
// highlighting, a block outline, a new-site template wizard, and
// validate/reload against a real Caddy instance — a local binary or a
// Docker container — instead of just editing text blind.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thelazurus/caddyedit-vibe/internal/app"
	"github.com/thelazurus/caddyedit-vibe/internal/dockerx"
	"github.com/thelazurus/caddyedit-vibe/internal/validate"
)

func main() {
	file := flag.String("file", "Caddyfile", "path to the Caddyfile to edit")
	socket := flag.String("socket", dockerx.DefaultSocket, "path to the Docker socket")
	container := flag.String("container", "", "name or ID of the Caddy container (auto-detected if omitted)")
	containerPath := flag.String("container-path", "/etc/caddy/Caddyfile", "path to the Caddyfile inside the container")
	noDocker := flag.Bool("no-docker", false, "skip Docker detection entirely")
	flag.Parse()

	content, err := os.ReadFile(*file)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "caddyedit:", err)
			os.Exit(1)
		}
		content = nil
	}

	target := resolveTarget(*socket, *container, *containerPath, *noDocker)

	m := app.New(*file, string(content), target)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "caddyedit:", err)
		os.Exit(1)
	}
}

func resolveTarget(socket, container, containerPath string, noDocker bool) validate.Target {
	if !noDocker {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client := dockerx.NewClient(socket)
		if client.Available(ctx) {
			if container != "" {
				return validate.Target{
					Kind: validate.TargetDocker, Docker: client,
					ContainerID: container, ContainerName: container, ContainerPath: containerPath,
				}
			}
			matches, err := client.FindByNameOrImage(ctx, "caddy")
			if err == nil && len(matches) == 1 {
				return validate.Target{
					Kind: validate.TargetDocker, Docker: client,
					ContainerID: matches[0].ID, ContainerName: matches[0].Name(), ContainerPath: containerPath,
				}
			}
			if err == nil && len(matches) > 1 {
				fmt.Fprintln(os.Stderr, "caddyedit: multiple candidate Caddy containers found, pass -container to pick one:")
				for _, c := range matches {
					fmt.Fprintf(os.Stderr, "  - %s (%s)\n", c.Name(), c.Image)
				}
			}
		}
	}

	if bin := validate.ResolveLocalBinary(); bin != "" {
		return validate.Target{Kind: validate.TargetLocalBinary, LocalBinary: bin}
	}

	return validate.Target{Kind: validate.TargetNone}
}
