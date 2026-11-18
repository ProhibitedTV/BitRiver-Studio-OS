package obsws

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/events"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

func TestClientSnapshotAndCommands(t *testing.T) {
	t.Parallel()

	server := newFakeOBSServer(t)
	defer server.Close()

	client := New(server.URL(), server.password,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithTimeouts(500*time.Millisecond, 500*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	version, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version.OBSVersion != "30.2.0" {
		t.Fatalf("unexpected obs version: %s", version.OBSVersion)
	}
	if !version.AuthenticationRequired {
		t.Fatal("expected authentication to be required")
	}

	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.ActiveScene != "Starting Soon" {
		t.Fatalf("unexpected active scene: %s", snapshot.ActiveScene)
	}
	if len(snapshot.Scenes) != 3 {
		t.Fatalf("unexpected scene count: %d", len(snapshot.Scenes))
	}
	if snapshot.Inputs[2].HasAudio {
		t.Fatal("expected browser overlay input to be treated as non-audio")
	}

	if err := client.SetCurrentScene(ctx, "Host Camera"); err != nil {
		t.Fatalf("set current scene: %v", err)
	}
	currentScene, err := client.CurrentProgramScene(ctx)
	if err != nil {
		t.Fatalf("current program scene: %v", err)
	}
	if currentScene != "Host Camera" {
		t.Fatalf("unexpected current scene: %s", currentScene)
	}

	muted, err := client.ToggleInputMuted(ctx, "Host Mic")
	if err != nil {
		t.Fatalf("toggle input mute: %v", err)
	}
	if !muted {
		t.Fatal("expected host mic to become muted")
	}

	if err := client.StartStream(ctx); err != nil {
		t.Fatalf("start stream: %v", err)
	}
	stream, err := client.StreamStatus(ctx)
	if err != nil {
		t.Fatalf("stream status: %v", err)
	}
	if !stream.Active {
		t.Fatal("expected stream to be active")
	}

	if err := client.StartRecording(ctx); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	record, err := client.RecordStatus(ctx)
	if err != nil {
		t.Fatalf("record status: %v", err)
	}
	if !record.Active {
		t.Fatal("expected recording to be active")
	}
	if record.Filename == "" {
		t.Fatal("expected normalized record filename")
	}
}

func TestClientPublishesNormalizedEvents(t *testing.T) {
	t.Parallel()

	server := newFakeOBSServer(t)
	server.setSessionEvents(1,
		fakeEvent("CurrentProgramSceneChanged", map[string]any{"sceneName": "Host Camera"}),
		fakeEvent("InputMuteStateChanged", map[string]any{"inputName": "Host Mic", "inputMuted": true}),
	)
	defer server.Close()

	hub := events.NewHub()
	subscription := hub.Subscribe()
	defer hub.Unsubscribe(subscription)

	client := New(server.URL(), server.password,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithEventPublisher(hub),
		WithReconnect(50*time.Millisecond, 100*time.Millisecond),
		WithTimeouts(500*time.Millisecond, 500*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Run(ctx)

	sceneEvent := waitForEvent(t, subscription, func(event model.EventEnvelope) bool {
		return event.Type == "obs.scene.current_program_changed"
	})
	if sceneEvent.Type != "obs.scene.current_program_changed" {
		t.Fatalf("unexpected event type: %s", sceneEvent.Type)
	}

	inputEvent := waitForEvent(t, subscription, func(event model.EventEnvelope) bool {
		return event.Type == "obs.input.mute_changed"
	})
	if inputEvent.Type != "obs.input.mute_changed" {
		t.Fatalf("unexpected event type: %s", inputEvent.Type)
	}

	currentScene, err := client.CurrentProgramScene(context.Background())
	if err != nil {
		t.Fatalf("current scene after event: %v", err)
	}
	if currentScene != "Host Camera" {
		t.Fatalf("unexpected current scene after event: %s", currentScene)
	}
}

func TestClientReconnectsAfterSessionDrop(t *testing.T) {
	t.Parallel()

	server := newFakeOBSServer(t)
	server.closeSessionAfterIdentify(1)
	server.setSessionEvents(2, fakeEvent("CurrentProgramSceneChanged", map[string]any{"sceneName": "Reconnect Scene"}))
	defer server.Close()

	hub := events.NewHub()
	subscription := hub.Subscribe()
	defer hub.Unsubscribe(subscription)

	client := New(server.URL(), server.password,
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithEventPublisher(hub),
		WithReconnect(25*time.Millisecond, 50*time.Millisecond),
		WithTimeouts(300*time.Millisecond, 300*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Run(ctx)

	event := waitForEvent(t, subscription, func(envelope model.EventEnvelope) bool {
		return envelope.Type == "obs.scene.current_program_changed"
	})
	if event.Type != "obs.scene.current_program_changed" {
		t.Fatalf("unexpected event type: %s", event.Type)
	}

	if connections := server.sessionConnections(); connections < 2 {
		t.Fatalf("expected at least 2 session connections, got %d", connections)
	}
}

func TestClientFailsAuthWithWrongPassword(t *testing.T) {
	t.Parallel()

	server := newFakeOBSServer(t)
	defer server.Close()

	client := New(server.URL(), "wrong-password",
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		WithTimeouts(300*time.Millisecond, 300*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := client.Version(ctx); err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("expected authentication failure, got %v", err)
	}
}

type fakeOBSServer struct {
	t                   *testing.T
	listener            net.Listener
	password            string
	challenge           string
	salt                string
	mu                  sync.Mutex
	sessionCount        int
	closeAfterSession   map[int]bool
	sessionEventPlan    map[int][]map[string]any
	currentProgramScene string
	scenes              []string
	lastBrowserURL      string
	failCreateInput     bool
	browserPluginMissing bool
	streamActive        bool
	recordActive        bool
	recordPath          string
	recordPaused        bool
	inputs              []fakeInput
}

type fakeInput struct {
	Name     string
	Kind     string
	HasAudio bool
	Muted    bool
	GainDB   float64
}

func newFakeOBSServer(t *testing.T) *fakeOBSServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := &fakeOBSServer{
		t:                   t,
		listener:            listener,
		password:            "bitriver-secret",
		challenge:           "challenge-token",
		salt:                "salt-token",
		closeAfterSession:   make(map[int]bool),
		sessionEventPlan:    make(map[int][]map[string]any),
		currentProgramScene: "Starting Soon",
		scenes: []string{"Starting Soon", "Host Camera", "Host + Guest"},
		recordPath:          "/data/recordings/bitriver-session-001.mkv",
		inputs: []fakeInput{
			{Name: "Host Mic", Kind: "pulse_input_capture", HasAudio: true, Muted: false, GainDB: 0},
			{Name: "Game Mix", Kind: "pulse_input_capture", HasAudio: true, Muted: false, GainDB: -2.5},
			{Name: "Browser Overlay", Kind: "browser_source", HasAudio: false, Muted: false, GainDB: 0},
		},
	}

	go server.serve()
	return server
}

func (server *fakeOBSServer) URL() string {
	return "ws://" + server.listener.Addr().String()
}

func (server *fakeOBSServer) Close() {
	_ = server.listener.Close()
}

func (server *fakeOBSServer) setSessionEvents(session int, events ...map[string]any) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.sessionEventPlan[session] = append([]map[string]any(nil), events...)
}

func (server *fakeOBSServer) closeSessionAfterIdentify(session int) {
	server.mu.Lock()
	defer server.mu.Unlock()
	server.closeAfterSession[session] = true
}

func (server *fakeOBSServer) sessionConnections() int {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.sessionCount
}

func (server *fakeOBSServer) serve() {
	for {
		conn, err := server.listener.Accept()
		if err != nil {
			return
		}
		go server.handleConnection(conn)
	}
}

func (server *fakeOBSServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	request, err := http.ReadRequest(reader)
	if err != nil {
		return
	}
	_ = request.Body.Close()

	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: test\r\n\r\n"
	if _, err := io.WriteString(conn, response); err != nil {
		return
	}

	hello := map[string]any{
		"op": 0,
		"d": map[string]any{
			"obsWebSocketVersion": "5.4.0",
			"rpcVersion":          1,
			"authentication": map[string]any{
				"challenge": server.challenge,
				"salt":      server.salt,
			},
		},
	}
	if err := server.writeJSON(conn, hello, false); err != nil {
		return
	}

	identifyPayload, err := server.readJSON(reader)
	if err != nil {
		return
	}

	var identify struct {
		Op int `json:"op"`
		D  struct {
			Authentication     string `json:"authentication"`
			EventSubscriptions int    `json:"eventSubscriptions"`
		} `json:"d"`
	}
	if err := json.Unmarshal(identifyPayload, &identify); err != nil {
		return
	}

	expectedAuth := authToken(server.password, server.salt, server.challenge)
	if identify.D.Authentication != expectedAuth {
		return
	}

	if err := server.writeJSON(conn, map[string]any{"op": 2, "d": map[string]any{"negotiatedRpcVersion": 1}}, false); err != nil {
		return
	}

	if identify.D.EventSubscriptions != 0 {
		server.handleSessionConnection(conn)
		return
	}

	server.handleRequestConnection(conn, reader)
}

func (server *fakeOBSServer) handleSessionConnection(conn net.Conn) {
	server.mu.Lock()
	server.sessionCount++
	sessionNumber := server.sessionCount
	events := append([]map[string]any(nil), server.sessionEventPlan[sessionNumber]...)
	shouldClose := server.closeAfterSession[sessionNumber]
	server.mu.Unlock()

	if shouldClose {
		return
	}

	for _, event := range events {
		time.Sleep(30 * time.Millisecond)
		if err := server.writeJSON(conn, event, false); err != nil {
			return
		}
	}

	buffer := make([]byte, 1)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		if _, err := conn.Read(buffer); err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return
		}
	}
}

func (server *fakeOBSServer) handleRequestConnection(conn net.Conn, reader *bufio.Reader) {
	for {
		payload, err := server.readJSON(reader)
		if err != nil {
			return
		}

		var request struct {
			Op int `json:"op"`
			D  struct {
				RequestType string         `json:"requestType"`
				RequestID   string         `json:"requestId"`
				RequestData map[string]any `json:"requestData"`
			} `json:"d"`
		}
		if err := json.Unmarshal(payload, &request); err != nil {
			return
		}

		responseData, responseErr := server.handleRequest(request.D.RequestType, request.D.RequestData)
		result := responseErr == nil
		comment := ""
		if responseErr != nil {
			comment = responseErr.Error()
		}

		response := map[string]any{
			"op": 7,
			"d": map[string]any{
				"requestId": request.D.RequestID,
				"requestStatus": map[string]any{
					"result":  result,
					"comment": comment,
				},
				"responseData": responseData,
			},
		}
		if err := server.writeJSON(conn, response, false); err != nil {
			return
		}
	}
}

func (server *fakeOBSServer) handleRequest(requestType string, requestData map[string]any) (map[string]any, error) {
	server.mu.Lock()
	defer server.mu.Unlock()

	switch requestType {
	case "GetVersion":
		return map[string]any{
			"obsVersion":          "30.2.0",
			"obsWebSocketVersion": "5.4.0",
			"platform":            "linux",
			"platformDescription": "BitRiver fake OBS",
			"rpcVersion":          1,
			"availableRequests": []string{
				"GetVersion",
				"GetSceneList",
				"CreateScene",
				"CreateInput",
				"RemoveScene",
				"GetInputList",
				"GetInputMute",
				"GetInputVolume",
				"GetStreamStatus",
				"GetRecordStatus",
				"SetCurrentProgramScene",
				"SetInputMute",
				"ToggleInputMute",
				"SetInputVolume",
				"StartStream",
				"StopStream",
				"StartRecord",
				"StopRecord",
			},
			"availableEvents": []string{
				"CurrentProgramSceneChanged",
				"InputMuteStateChanged",
				"InputVolumeChanged",
				"StreamStateChanged",
				"RecordStateChanged",
			},
		}, nil
	case "GetSceneList":
		scenes := make([]map[string]any, 0, len(server.scenes))
		for i, name := range server.scenes {
			scenes = append(scenes, map[string]any{"sceneIndex": i, "sceneName": name})
		}
		return map[string]any{"currentProgramSceneName": server.currentProgramScene, "scenes": scenes}, nil
	case "CreateScene":
		name := stringValue(requestData["sceneName"])
		for _, existing := range server.scenes {
			if existing == name {
				return nil, errors.New("scene already exists")
			}
		}
		server.scenes = append(server.scenes, name)
		return map[string]any{}, nil
	case "RemoveScene":
		name := stringValue(requestData["sceneName"])
		for i, existing := range server.scenes {
			if existing == name && name != server.currentProgramScene {
				server.scenes = append(server.scenes[:i], server.scenes[i+1:]...)
				return map[string]any{}, nil
			}
		}
		return nil, errors.New("scene cannot be removed")
	case "CreateInput":
		if server.failCreateInput { return nil, errors.New("browser source unavailable") }
		sceneName := stringValue(requestData["sceneName"])
		foundScene := false
		for _, name := range server.scenes { if name == sceneName { foundScene = true } }
		if !foundScene { return nil, errors.New("target scene not found") }
		name := stringValue(requestData["inputName"])
		for _, input := range server.inputs {
			if input.Name == name { return nil, errors.New("input already exists") }
		}
		if stringValue(requestData["inputKind"]) != "browser_source" { return nil, errors.New("unsupported source kind") }
		settings, ok := requestData["inputSettings"].(map[string]any)
		if !ok { return nil, errors.New("missing input settings") }
		server.lastBrowserURL = stringValue(settings["url"])
		server.inputs = append(server.inputs, fakeInput{Name: name, Kind: "browser_source"})
		return map[string]any{"sceneItemId": 12}, nil
	case "GetInputKindList":
		if server.browserPluginMissing {
			return map[string]any{"inputKinds": []string{"pulse_input_capture"}}, nil
		}
		return map[string]any{"inputKinds": []string{"pulse_input_capture", "browser_source"}}, nil
	case "GetInputList":
		inputs := make([]map[string]any, 0, len(server.inputs))
		for _, input := range server.inputs {
			inputs = append(inputs, map[string]any{
				"inputName": input.Name,
				"inputKind": input.Kind,
			})
		}
		return map[string]any{"inputs": inputs}, nil
	case "GetInputMute":
		input, err := server.lookupInput(requestData["inputName"])
		if err != nil {
			return nil, err
		}
		if !input.HasAudio {
			return nil, errors.New("input does not have audio")
		}
		return map[string]any{"inputMuted": input.Muted}, nil
	case "GetInputVolume":
		input, err := server.lookupInput(requestData["inputName"])
		if err != nil {
			return nil, err
		}
		if !input.HasAudio {
			return nil, errors.New("input does not have audio")
		}
		return map[string]any{"inputVolumeDb": input.GainDB}, nil
	case "SetCurrentProgramScene":
		server.currentProgramScene = stringValue(requestData["sceneName"])
		return map[string]any{}, nil
	case "SetInputMute":
		input, err := server.lookupInput(requestData["inputName"])
		if err != nil {
			return nil, err
		}
		input.Muted = boolValue(requestData["inputMuted"])
		return map[string]any{}, nil
	case "ToggleInputMute":
		input, err := server.lookupInput(requestData["inputName"])
		if err != nil {
			return nil, err
		}
		input.Muted = !input.Muted
		return map[string]any{}, nil
	case "SetInputVolume":
		input, err := server.lookupInput(requestData["inputName"])
		if err != nil {
			return nil, err
		}
		input.GainDB = floatValue(requestData["inputVolumeDb"])
		return map[string]any{}, nil
	case "GetStreamStatus":
		return map[string]any{
			"outputActive":        server.streamActive,
			"outputReconnecting":  false,
			"outputTimecode":      "00:00:10.000",
			"outputDuration":      10000,
			"outputBytes":         int64(1200000),
			"outputSkippedFrames": int64(0),
			"outputTotalFrames":   int64(300),
		}, nil
	case "StartStream":
		server.streamActive = true
		return map[string]any{}, nil
	case "StopStream":
		server.streamActive = false
		return map[string]any{}, nil
	case "GetRecordStatus":
		return map[string]any{
			"outputActive":   server.recordActive,
			"outputPaused":   server.recordPaused,
			"outputPath":     server.recordPath,
			"outputTimecode": "00:00:10.000",
			"outputDuration": 10000,
			"outputBytes":    int64(6400000),
		}, nil
	case "StartRecord":
		server.recordActive = true
		server.recordPath = "/data/recordings/bitriver-session-002.mkv"
		return map[string]any{}, nil
	case "StopRecord":
		server.recordActive = false
		return map[string]any{}, nil
	default:
		return nil, errors.New("unsupported request: " + requestType)
	}
}

func (server *fakeOBSServer) lookupInput(value any) (*fakeInput, error) {
	name := stringValue(value)
	for index := range server.inputs {
		if server.inputs[index].Name == name {
			return &server.inputs[index], nil
		}
	}
	return nil, errors.New("input not found")
}

func (server *fakeOBSServer) readJSON(reader io.Reader) ([]byte, error) {
	for {
		opcode, payload, err := readFrame(reader)
		if err != nil {
			return nil, err
		}
		if opcode == 0x1 {
			return payload, nil
		}
	}
}

func (server *fakeOBSServer) writeJSON(writer io.Writer, payload any, mask bool) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writeFrame(writer, 0x1, data, mask)
}

func fakeEvent(eventType string, eventData map[string]any) map[string]any {
	return map[string]any{
		"op": 5,
		"d": map[string]any{
			"eventType": eventType,
			"eventData": eventData,
		},
	}
}

func waitForEvent(t *testing.T, subscription chan []byte, match func(model.EventEnvelope) bool) model.EventEnvelope {
	t.Helper()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case payload := <-subscription:
			var event model.EventEnvelope
			if err := json.Unmarshal(payload, &event); err != nil {
				t.Fatalf("decode event: %v", err)
			}
			if match(event) {
				return event
			}
		case <-deadline:
			t.Fatal("timed out waiting for OBS event")
		}
	}
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func boolValue(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	return false
}

func floatValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	default:
		return 0
	}
}
