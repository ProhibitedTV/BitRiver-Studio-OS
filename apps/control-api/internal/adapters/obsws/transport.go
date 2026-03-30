package obsws

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
)

type helloInfo struct {
	AuthenticationRequired bool
}

type messageEnvelope struct {
	Op   int `json:"op"`
	Data struct {
		EventType string          `json:"eventType"`
		EventData json.RawMessage `json:"eventData"`
	} `json:"d"`
}

type requestConnection struct {
	conn         net.Conn
	reader       *bufio.Reader
	authRequired bool
}

func (client *Client) connectRequest(ctx context.Context) (*requestConnection, error) {
	connection, _, err := client.connect(ctx, 0)
	if err != nil {
		return nil, err
	}
	return connection, nil
}

func (client *Client) connect(ctx context.Context, eventSubscriptions int) (*requestConnection, helloInfo, error) {
	target, err := normalizeURL(client.endpoint)
	if err != nil {
		return nil, helloInfo{}, err
	}

	dialer := &net.Dialer{Timeout: client.connectTimeout}
	rawConn, err := dialer.DialContext(ctx, "tcp", target.Host)
	if err != nil {
		return nil, helloInfo{}, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = rawConn.SetDeadline(deadline)
	} else {
		_ = rawConn.SetDeadline(time.Now().Add(client.connectTimeout))
	}

	if err := writeHandshake(rawConn, target); err != nil {
		_ = rawConn.Close()
		return nil, helloInfo{}, err
	}

	reader := bufio.NewReader(rawConn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	if err != nil {
		_ = rawConn.Close()
		return nil, helloInfo{}, err
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		_ = rawConn.Close()
		return nil, helloInfo{}, fmt.Errorf("obs-websocket handshake failed with %s", response.Status)
	}

	connection := &requestConnection{
		conn:   rawConn,
		reader: reader,
	}

	var hello struct {
		Op int `json:"op"`
		D  struct {
			Authentication struct {
				Challenge string `json:"challenge"`
				Salt      string `json:"salt"`
			} `json:"authentication"`
		} `json:"d"`
	}
	if err := connection.readJSON(&hello); err != nil {
		connection.close()
		return nil, helloInfo{}, err
	}
	if hello.Op != 0 {
		connection.close()
		return nil, helloInfo{}, errors.New("obs-websocket did not send hello")
	}

	identify := map[string]any{
		"op": 1,
		"d": map[string]any{
			"rpcVersion": 1,
		},
	}
	authRequired := hello.D.Authentication.Challenge != "" && hello.D.Authentication.Salt != ""
	connection.authRequired = authRequired
	if authRequired {
		identify["d"].(map[string]any)["authentication"] = authToken(client.password, hello.D.Authentication.Salt, hello.D.Authentication.Challenge)
	}
	if eventSubscriptions > 0 {
		identify["d"].(map[string]any)["eventSubscriptions"] = eventSubscriptions
	}

	if err := connection.writeJSON(identify); err != nil {
		connection.close()
		return nil, helloInfo{}, err
	}

	var identified struct {
		Op int `json:"op"`
	}
	if err := connection.readJSON(&identified); err != nil {
		connection.close()
		return nil, helloInfo{}, classifyAuthFailure(err)
	}
	if identified.Op != 2 {
		connection.close()
		return nil, helloInfo{}, errors.New("obs-websocket identify failed")
	}

	if eventSubscriptions > 0 {
		_ = rawConn.SetDeadline(time.Time{})
	}

	return connection, helloInfo{AuthenticationRequired: authRequired}, nil
}

func (connection *requestConnection) close() {
	_ = connection.conn.Close()
}

func (connection *requestConnection) request(requestType string, requestData any) (json.RawMessage, error) {
	requestID := fmt.Sprintf("req-%d", atomic.AddUint64(&requestSequence, 1))
	payload := map[string]any{
		"op": 6,
		"d": map[string]any{
			"requestType": requestType,
			"requestId":   requestID,
		},
	}
	if requestData != nil {
		payload["d"].(map[string]any)["requestData"] = requestData
	}
	if err := connection.writeJSON(payload); err != nil {
		return nil, err
	}

	for {
		var response struct {
			Op int `json:"op"`
			D  struct {
				RequestID     string `json:"requestId"`
				RequestStatus struct {
					Result  bool   `json:"result"`
					Comment string `json:"comment"`
				} `json:"requestStatus"`
				ResponseData json.RawMessage `json:"responseData"`
			} `json:"d"`
		}
		if err := connection.readJSON(&response); err != nil {
			return nil, err
		}
		if response.Op != 7 || response.D.RequestID != requestID {
			continue
		}
		if !response.D.RequestStatus.Result {
			return nil, fmt.Errorf("%s failed: %s", requestType, response.D.RequestStatus.Comment)
		}
		return response.D.ResponseData, nil
	}
}

func (connection *requestConnection) readEnvelope() (messageEnvelope, error) {
	var envelope messageEnvelope
	for {
		opcode, payload, err := readFrame(connection.reader)
		if err != nil {
			return envelope, err
		}
		switch opcode {
		case 0x1:
			if err := json.Unmarshal(payload, &envelope); err != nil {
				return envelope, err
			}
			return envelope, nil
		case 0x8:
			return envelope, io.EOF
		case 0x9:
			if err := writeFrame(connection.conn, 0xA, payload, true); err != nil {
				return envelope, err
			}
		}
	}
}

func (connection *requestConnection) writeJSON(payload any) error {
	frame, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writeFrame(connection.conn, 0x1, frame, true)
}

func (connection *requestConnection) readJSON(target any) error {
	for {
		opcode, payload, err := readFrame(connection.reader)
		if err != nil {
			return err
		}
		switch opcode {
		case 0x1:
			return json.Unmarshal(payload, target)
		case 0x8:
			return io.EOF
		case 0x9:
			if err := writeFrame(connection.conn, 0xA, payload, true); err != nil {
				return err
			}
		}
	}
}

func buildSnapshot(connection *requestConnection) (adapters.OBSSnapshot, error) {
	version, err := getVersion(connection)
	if err != nil {
		return adapters.OBSSnapshot{}, err
	}
	version.AuthenticationRequired = connection.authRequired
	sceneList, activeScene, err := getScenes(connection)
	if err != nil {
		return adapters.OBSSnapshot{}, err
	}
	streamStatus, err := getStreamStatus(connection)
	if err != nil {
		return adapters.OBSSnapshot{}, err
	}
	recordStatus, err := getRecordStatus(connection)
	if err != nil {
		return adapters.OBSSnapshot{}, err
	}
	inputs, err := getInputs(connection)
	if err != nil {
		return adapters.OBSSnapshot{}, err
	}

	now := time.Now().UTC()
	return adapters.OBSSnapshot{
		Version:         version,
		Scenes:          sceneList,
		ActiveScene:     activeScene,
		StreamStatus:    streamStatus,
		RecordStatus:    recordStatus,
		StreamActive:    streamStatus.Active,
		RecordingActive: recordStatus.Active,
		RecordingPath:   recordStatus.Path,
		RecordingFile:   recordStatus.Filename,
		Inputs:          inputs,
		LastChangedAt:   &now,
	}, nil
}

func getVersion(connection *requestConnection) (adapters.OBSVersion, error) {
	payload, err := connection.request("GetVersion", nil)
	if err != nil {
		return adapters.OBSVersion{}, err
	}

	var response struct {
		OBSVersion          string   `json:"obsVersion"`
		ObsWebSocketVersion string   `json:"obsWebSocketVersion"`
		Platform            string   `json:"platform"`
		PlatformDescription string   `json:"platformDescription"`
		RPCVersion          int      `json:"rpcVersion"`
		AvailableRequests   []string `json:"availableRequests"`
		AvailableEvents     []string `json:"availableEvents"`
	}
	if err := decodeJSON(payload, &response); err != nil {
		return adapters.OBSVersion{}, err
	}

	return adapters.OBSVersion{
		OBSVersion:          response.OBSVersion,
		WebSocketVersion:    response.ObsWebSocketVersion,
		Platform:            response.Platform,
		PlatformDescription: response.PlatformDescription,
		RPCVersion:          response.RPCVersion,
		AvailableRequests:   append([]string(nil), response.AvailableRequests...),
		AvailableEvents:     append([]string(nil), response.AvailableEvents...),
	}, nil
}

func getScenes(connection *requestConnection) ([]adapters.OBSScene, string, error) {
	payload, err := connection.request("GetSceneList", nil)
	if err != nil {
		return nil, "", err
	}

	var response struct {
		CurrentProgramSceneName string `json:"currentProgramSceneName"`
		Scenes                  []struct {
			SceneIndex int    `json:"sceneIndex"`
			SceneName  string `json:"sceneName"`
		} `json:"scenes"`
	}
	if err := decodeJSON(payload, &response); err != nil {
		return nil, "", err
	}

	scenes := make([]adapters.OBSScene, 0, len(response.Scenes))
	for _, scene := range response.Scenes {
		scenes = append(scenes, adapters.OBSScene{
			Name:   scene.SceneName,
			Index:  scene.SceneIndex,
			Active: scene.SceneName == response.CurrentProgramSceneName,
		})
	}

	return scenes, response.CurrentProgramSceneName, nil
}

func getInputs(connection *requestConnection) ([]adapters.OBSInput, error) {
	payload, err := connection.request("GetInputList", nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Inputs []struct {
			InputName string `json:"inputName"`
			InputKind string `json:"inputKind"`
		} `json:"inputs"`
	}
	if err := decodeJSON(payload, &response); err != nil {
		return nil, err
	}

	inputs := make([]adapters.OBSInput, 0, len(response.Inputs))
	for _, input := range response.Inputs {
		summary := adapters.OBSInput{
			Name:      input.InputName,
			Kind:      input.InputKind,
			Available: true,
		}

		var muteResponse struct {
			InputMuted bool `json:"inputMuted"`
		}
		mutePayload, muteErr := connection.request("GetInputMute", map[string]any{"inputName": input.InputName})
		if muteErr == nil {
			if err := decodeJSON(mutePayload, &muteResponse); err != nil {
				return nil, err
			}
			summary.HasAudio = true
			summary.Muted = muteResponse.InputMuted
		}

		var volumeResponse struct {
			InputVolumeDb float64 `json:"inputVolumeDb"`
		}
		volumePayload, volumeErr := connection.request("GetInputVolume", map[string]any{"inputName": input.InputName})
		if volumeErr == nil {
			if err := decodeJSON(volumePayload, &volumeResponse); err != nil {
				return nil, err
			}
			summary.HasAudio = true
			summary.GainDB = volumeResponse.InputVolumeDb
			summary.PeakDB = volumeResponse.InputVolumeDb - 3
		}

		if muteErr != nil && volumeErr != nil {
			summary.HasAudio = false
			summary.Muted = false
			summary.GainDB = 0
			summary.PeakDB = -60
		}

		now := time.Now().UTC()
		summary.LastChangedAt = &now
		inputs = append(inputs, summary)
	}

	return inputs, nil
}

func getStreamStatus(connection *requestConnection) (adapters.OBSStreamStatus, error) {
	payload, err := connection.request("GetStreamStatus", nil)
	if err != nil {
		return adapters.OBSStreamStatus{}, err
	}

	var response struct {
		OutputActive        bool    `json:"outputActive"`
		OutputReconnecting  bool    `json:"outputReconnecting"`
		OutputTimecode      string  `json:"outputTimecode"`
		OutputDuration      float64 `json:"outputDuration"`
		OutputBytes         int64   `json:"outputBytes"`
		OutputSkippedFrames int64   `json:"outputSkippedFrames"`
		OutputTotalFrames   int64   `json:"outputTotalFrames"`
	}
	if err := decodeJSON(payload, &response); err != nil {
		return adapters.OBSStreamStatus{}, err
	}

	now := time.Now().UTC()
	return adapters.OBSStreamStatus{
		Active:          response.OutputActive,
		Reconnecting:    response.OutputReconnecting,
		Timecode:        response.OutputTimecode,
		DurationSeconds: int64(response.OutputDuration / 1000),
		BytesPerSecond:  response.OutputBytes,
		FramesSkipped:   response.OutputSkippedFrames,
		FramesTotal:     response.OutputTotalFrames,
		LastChangedAt:   &now,
	}, nil
}

func getRecordStatus(connection *requestConnection) (adapters.OBSRecordStatus, error) {
	payload, err := connection.request("GetRecordStatus", nil)
	if err != nil {
		return adapters.OBSRecordStatus{}, err
	}

	var response struct {
		OutputActive   bool    `json:"outputActive"`
		OutputPaused   bool    `json:"outputPaused"`
		OutputPath     string  `json:"outputPath"`
		OutputTimecode string  `json:"outputTimecode"`
		OutputDuration float64 `json:"outputDuration"`
		OutputBytes    int64   `json:"outputBytes"`
	}
	if err := decodeJSON(payload, &response); err != nil {
		return adapters.OBSRecordStatus{}, err
	}

	now := time.Now().UTC()
	return adapters.OBSRecordStatus{
		Active:          response.OutputActive,
		Paused:          response.OutputPaused,
		Path:            recordDirectory(response.OutputPath),
		Filename:        recordFilename(response.OutputPath),
		Timecode:        response.OutputTimecode,
		DurationSeconds: int64(response.OutputDuration / 1000),
		Bytes:           response.OutputBytes,
		LastChangedAt:   &now,
	}, nil
}

func normalizeURL(endpoint string) (*url.URL, error) {
	value := endpoint
	if value == "" {
		value = "ws://127.0.0.1:4455"
	}
	if !strings.Contains(value, "://") {
		value = "ws://" + value
	}
	target, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	if target.Path == "" {
		target.Path = "/"
	}
	return target, nil
}

func writeHandshake(writer io.Writer, target *url.URL) error {
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		return err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", target.RequestURI(), target.Host, key)
	_, err := io.WriteString(writer, request)
	return err
}

func authToken(password string, salt string, challenge string) string {
	secretSum := sha256.Sum256([]byte(password + salt))
	secret := base64.StdEncoding.EncodeToString(secretSum[:])
	authSum := sha256.Sum256([]byte(secret + challenge))
	return base64.StdEncoding.EncodeToString(authSum[:])
}

var requestSequence uint64

func readFrame(reader io.Reader) (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}

	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	payloadLen := uint64(header[1] & 0x7F)
	switch payloadLen {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return 0, nil, err
		}
		payloadLen = uint64(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return 0, nil, err
		}
		payloadLen = binary.BigEndian.Uint64(extended)
	}

	maskKey := make([]byte, 4)
	if masked {
		if _, err := io.ReadFull(reader, maskKey); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for index := range payload {
			payload[index] ^= maskKey[index%4]
		}
	}

	return opcode, payload, nil
}

func writeFrame(writer io.Writer, opcode byte, payload []byte, mask bool) error {
	header := []byte{0x80 | opcode}
	maskBit := byte(0)
	if mask {
		maskBit = 0x80
	}

	switch {
	case len(payload) < 126:
		header = append(header, maskBit|byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, maskBit|126)
		length := make([]byte, 2)
		binary.BigEndian.PutUint16(length, uint16(len(payload)))
		header = append(header, length...)
	default:
		header = append(header, maskBit|127)
		length := make([]byte, 8)
		binary.BigEndian.PutUint64(length, uint64(len(payload)))
		header = append(header, length...)
	}

	if _, err := writer.Write(header); err != nil {
		return err
	}

	if mask {
		maskKey := make([]byte, 4)
		if _, err := rand.Read(maskKey); err != nil {
			return err
		}
		if _, err := writer.Write(maskKey); err != nil {
			return err
		}
		maskedPayload := append([]byte(nil), payload...)
		for index := range maskedPayload {
			maskedPayload[index] ^= maskKey[index%4]
		}
		_, err := writer.Write(maskedPayload)
		return err
	}

	_, err := writer.Write(payload)
	return err
}

func classifyAuthFailure(err error) error {
	if errors.Is(err, io.EOF) {
		return errors.New("obs-websocket authentication failed or the connection was closed during identify")
	}
	return err
}
