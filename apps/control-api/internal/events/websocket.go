package events

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

func ServeWebsocket(writer http.ResponseWriter, request *http.Request, hub *Hub, initialEvent model.EventEnvelope) {
	if !isWebsocketRequest(request) {
		http.Error(writer, "websocket upgrade required", http.StatusBadRequest)
		return
	}

	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		http.Error(writer, "websocket upgrade not supported", http.StatusInternalServerError)
		return
	}

	conn, buffer, err := hijacker.Hijack()
	if err != nil {
		http.Error(writer, "websocket upgrade failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	key := request.Header.Get("Sec-WebSocket-Key")
	accept := websocketAccept(key)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := io.WriteString(buffer, response); err != nil {
		return
	}
	if err := buffer.Flush(); err != nil {
		return
	}

	subscription := hub.Subscribe()
	defer hub.Unsubscribe(subscription)

	if payload, err := marshalEvent(initialEvent); err == nil {
		if err := writeServerFrame(conn, 0x1, payload); err != nil {
			return
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		reader := bufio.NewReader(conn)
		for {
			opcode, payload, err := readClientFrame(reader)
			if err != nil {
				return
			}
			if opcode == 0x8 {
				return
			}
			if opcode == 0x9 {
				_ = writeServerFrame(conn, 0xA, payload)
			}
		}
	}()

	for {
		select {
		case payload, ok := <-subscription:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := writeServerFrame(conn, 0x1, payload); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func isWebsocketRequest(request *http.Request) bool {
	return strings.EqualFold(request.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(request.Header.Get("Connection")), "upgrade")
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func marshalEvent(event model.EventEnvelope) ([]byte, error) {
	return jsonMarshal(event)
}

func readClientFrame(reader io.Reader) (byte, []byte, error) {
	return readFrame(reader)
}

func writeServerFrame(writer io.Writer, opcode byte, payload []byte) error {
	return writeFrame(writer, opcode, payload, false)
}

func readFrame(reader io.Reader) (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}

	opcode := header[0] & 0x0F
	mask := header[1]&0x80 != 0
	length, err := frameLength(reader, header[1]&0x7F)
	if err != nil {
		return 0, nil, err
	}

	maskKey := make([]byte, 4)
	if mask {
		if _, err := io.ReadFull(reader, maskKey); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return 0, nil, err
	}
	if mask {
		for index := range payload {
			payload[index] ^= maskKey[index%4]
		}
	}
	return opcode, payload, nil
}

func writeFrame(writer io.Writer, opcode byte, payload []byte, masked bool) error {
	header := []byte{0x80 | opcode}
	maskBit := byte(0)
	if masked {
		maskBit = 0x80
	}

	switch {
	case len(payload) < 126:
		header = append(header, maskBit|byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, maskBit|126, byte(len(payload)>>8), byte(len(payload)))
	default:
		header = append(header, maskBit|127,
			byte(uint64(len(payload))>>56),
			byte(uint64(len(payload))>>48),
			byte(uint64(len(payload))>>40),
			byte(uint64(len(payload))>>32),
			byte(uint64(len(payload))>>24),
			byte(uint64(len(payload))>>16),
			byte(uint64(len(payload))>>8),
			byte(uint64(len(payload))))
	}

	if _, err := writer.Write(header); err != nil {
		return err
	}

	if _, err := writer.Write(payload); err != nil {
		return err
	}
	return nil
}

func frameLength(reader io.Reader, short byte) (int, error) {
	switch short {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return 0, err
		}
		return int(extended[0])<<8 | int(extended[1]), nil
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return 0, err
		}
		var total uint64
		for _, value := range extended {
			total = (total << 8) | uint64(value)
		}
		return int(total), nil
	default:
		return int(short), nil
	}
}

func jsonMarshal(event model.EventEnvelope) ([]byte, error) {
	type alias model.EventEnvelope
	return json.Marshal(alias(event))
}
