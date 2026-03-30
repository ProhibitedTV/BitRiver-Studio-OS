package systemd

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
)

type Client struct{}

func New() *Client {
	return &Client{}
}

func (client *Client) Status(ctx context.Context, names []string) ([]adapters.UnitStatus, error) {
	statuses := make([]adapters.UnitStatus, 0, len(names))
	for _, name := range names {
		statuses = append(statuses, client.statusOne(ctx, name))
	}
	return statuses, nil
}

func (client *Client) statusOne(ctx context.Context, name string) adapters.UnitStatus {
	if runtime.GOOS != "linux" {
		return adapters.UnitStatus{
			Name:   name,
			Active: true,
			State:  "unknown",
			Detail: "systemd probing is only available on Linux hosts",
		}
	}

	command := exec.CommandContext(ctx, "systemctl", "show", name, "--property=ActiveState,SubState,LoadState,Result", "--no-page")
	output, err := command.Output()
	if err != nil {
		return adapters.UnitStatus{
			Name:   name,
			Active: false,
			State:  "unknown",
			Detail: err.Error(),
		}
	}

	fields := parseProperties(output)
	activeState := fields["ActiveState"]
	subState := fields["SubState"]
	result := fields["Result"]

	detail := subState
	if result != "" && result != "success" {
		detail = subState + " (" + result + ")"
	}

	return adapters.UnitStatus{
		Name:       name,
		Active:     activeState == "active",
		State:      activeState,
		Detail:     strings.TrimSpace(detail),
		Restarting: activeState == "activating",
	}
}

func parseProperties(payload []byte) map[string]string {
	fields := make(map[string]string)
	for _, line := range bytes.Split(payload, []byte{'\n'}) {
		pair := strings.SplitN(string(line), "=", 2)
		if len(pair) == 2 {
			fields[pair[0]] = pair[1]
		}
	}
	return fields
}
