// Package dockerx is a minimal Docker Engine API client, talking directly to
// the Docker socket over HTTP. It only implements what caddyedit needs:
// listing containers and running one-shot execs (for `caddy validate` /
// `caddy reload` inside a containerized Caddy).
package dockerx

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const DefaultSocket = "/var/run/docker.sock"

type Client struct {
	http *http.Client
}

func NewClient(socketPath string) *Client {
	if socketPath == "" {
		socketPath = DefaultSocket
	}
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					d := net.Dialer{}
					return d.DialContext(ctx, "unix", socketPath)
				},
			},
		},
	}
}

// Available checks whether the Docker socket is reachable at all.
func (c *Client) Available(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/_ping", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type Container struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	State string   `json:"State"`
}

func (c Container) Name() string {
	if len(c.Names) == 0 {
		return c.ID
	}
	return strings.TrimPrefix(c.Names[0], "/")
}

func (c *Client) ListRunningContainers(ctx context.Context) ([]Container, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/containers/json?all=false", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker socket unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("docker: list containers: %s: %s", resp.Status, body)
	}
	var out []Container
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindByNameOrImage returns running containers whose name or image contains
// the given substring (case-insensitive).
func (c *Client) FindByNameOrImage(ctx context.Context, substr string) ([]Container, error) {
	all, err := c.ListRunningContainers(ctx)
	if err != nil {
		return nil, err
	}
	substr = strings.ToLower(substr)
	var out []Container
	for _, ct := range all {
		if strings.Contains(strings.ToLower(ct.Name()), substr) || strings.Contains(strings.ToLower(ct.Image), substr) {
			out = append(out, ct)
		}
	}
	return out, nil
}

type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Exec runs cmd inside containerID and waits for it to finish, capturing
// stdout/stderr separately.
func (c *Client) Exec(ctx context.Context, containerID string, cmd []string) (ExecResult, error) {
	createBody, _ := json.Marshal(map[string]any{
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://unix/containers/"+containerID+"/exec", bytes.NewReader(createBody))
	if err != nil {
		return ExecResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return ExecResult{}, err
	}
	var created struct {
		Id string
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return ExecResult{}, fmt.Errorf("docker: exec create: %s: %s", resp.Status, body)
	}
	if err := json.Unmarshal(body, &created); err != nil {
		return ExecResult{}, err
	}

	startBody, _ := json.Marshal(map[string]any{"Detach": false, "Tty": false})
	req, err = http.NewRequestWithContext(ctx, http.MethodPost,
		"http://unix/exec/"+created.Id+"/start", bytes.NewReader(startBody))
	if err != nil {
		return ExecResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = c.http.Do(req)
	if err != nil {
		return ExecResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ExecResult{}, fmt.Errorf("docker: exec start: %s: %s", resp.Status, b)
	}
	stdout, stderr, err := demux(resp.Body)
	if err != nil {
		return ExecResult{}, err
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/exec/"+created.Id+"/json", nil)
	if err != nil {
		return ExecResult{}, err
	}
	resp, err = c.http.Do(req)
	if err != nil {
		return ExecResult{}, err
	}
	defer resp.Body.Close()
	var inspect struct {
		ExitCode int
	}
	if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
		return ExecResult{}, err
	}

	return ExecResult{Stdout: string(stdout), Stderr: string(stderr), ExitCode: inspect.ExitCode}, nil
}

// demux splits a Docker multiplexed stdout/stderr stream (used when the
// exec was started without a TTY) into separate buffers.
func demux(r io.Reader) (stdout, stderr []byte, err error) {
	header := make([]byte, 8)
	for {
		_, err = io.ReadFull(r, header)
		if err == io.EOF {
			return stdout, stderr, nil
		}
		if err != nil {
			return stdout, stderr, err
		}
		size := binary.BigEndian.Uint32(header[4:8])
		payload := make([]byte, size)
		if _, err = io.ReadFull(r, payload); err != nil {
			return stdout, stderr, err
		}
		switch header[0] {
		case 2:
			stderr = append(stderr, payload...)
		default:
			stdout = append(stdout, payload...)
		}
	}
}
