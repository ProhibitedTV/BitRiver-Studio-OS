package obsws

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
)

// InstallBrowserScene adds a reviewed local HTML asset to a *new*, inactive OBS
// scene. It never changes the program scene or starts streaming. Mutations are
// intentionally not retried: a lost websocket reply leaves the result unknown.
func (client *Client) InstallBrowserScene(ctx context.Context, sceneName, inputName, sourceURL string, width, height int) error {
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme != "file" || parsed.Host != "" ||
		!strings.HasPrefix(parsed.Path, "/") || !strings.HasSuffix(parsed.Path, ".html") ||
		sceneName == "" || inputName == "" || width < 1 || height < 1 || width > 7680 || height > 4320 {
		return fmt.Errorf("invalid local OBS browser source")
	}

	client.installMu.Lock()
	defer client.installMu.Unlock()

	requestCtx, cancel := context.WithTimeout(ctx, 10*client.requestTimeout)
	defer cancel()
	connection, err := client.connectRequest(requestCtx)
	if err != nil {
		return fmt.Errorf("connect to OBS: %w", err)
	}
	defer connection.close()

	for _, command := range []string{"GetStreamStatus", "GetRecordStatus"} {
		payload, err := connection.request(command, nil)
		if err != nil {
			return fmt.Errorf("check OBS output safety: %w", err)
		}
		var status struct { Active bool `json:"outputActive"` }
		if err := decodeJSON(payload, &status); err != nil {
			return fmt.Errorf("decode OBS output state: %w", err)
		}
		if status.Active {
			return adapters.ErrSceneInstallBusy
		}
	}

	// Check that OBS has loaded the browser plugin before creating any scene.
	// The API can be reachable even when browser_source is not registered.
	kindsJSON, err := connection.request("GetInputKindList", nil)
	if err != nil {
		return fmt.Errorf("check OBS Browser Source availability: %w", err)
	}
	var kinds struct { InputKinds []string `json:"inputKinds"` }
	if err := decodeJSON(kindsJSON, &kinds); err != nil {
		return fmt.Errorf("decode OBS available input kinds: %w", err)
	}
	browserReady := false
	for _, kind := range kinds.InputKinds {
		if kind == "browser_source" { browserReady = true; break }
	}
	if !browserReady {
		return adapters.ErrBrowserSourceUnavailable
	}

	scenesJSON, err := connection.request("GetSceneList", nil)
	if err != nil {
		return fmt.Errorf("check OBS scenes: %w", err)
	}
	var scenes struct { Scenes []struct { Name string `json:"sceneName"` } `json:"scenes"` }
	if err := decodeJSON(scenesJSON, &scenes); err != nil {
		return fmt.Errorf("decode OBS scenes: %w", err)
	}
	for _, scene := range scenes.Scenes {
		if scene.Name == sceneName {
			return adapters.ErrSceneInstallCollision
		}
	}
	inputsJSON, err := connection.request("GetInputList", nil)
	if err != nil {
		return fmt.Errorf("check OBS inputs: %w", err)
	}
	var inputs struct { Inputs []struct { Name string `json:"inputName"` } `json:"inputs"` }
	if err := decodeJSON(inputsJSON, &inputs); err != nil {
		return fmt.Errorf("decode OBS inputs: %w", err)
	}
	for _, input := range inputs.Inputs {
		if input.Name == inputName {
			return adapters.ErrSceneInstallCollision
		}
	}

	if _, err := connection.request("CreateScene", map[string]any{"sceneName": sceneName}); err != nil {
		return fmt.Errorf("create OBS scene: %w", err)
	}
	_, err = connection.request("CreateInput", map[string]any{
		"sceneName": sceneName,
		"inputName": inputName,
		"inputKind": "browser_source",
		"inputSettings": map[string]any{
			"url": sourceURL,
			"width": width,
			"height": height,
		},
		"sceneItemEnabled": true,
	})
	if err != nil {
		// Delete only the scene created by this request. Never delete an existing
		// user scene/input; if OBS disconnects, report uncertain cleanup.
		if _, rollbackErr := connection.request("RemoveScene", map[string]any{"sceneName": sceneName}); rollbackErr != nil {
			return fmt.Errorf("create OBS browser input failed (%v); rollback uncertain (%v); inspect OBS manually", err, rollbackErr)
		}
		return fmt.Errorf("create OBS browser input: %w (new scene rolled back)", err)
	}
	client.triggerRefresh()
	return nil
}

var _ adapters.BrowserSceneInstaller = (*Client)(nil)
