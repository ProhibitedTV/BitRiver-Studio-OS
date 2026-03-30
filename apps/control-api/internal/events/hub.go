package events

import (
	"encoding/json"
	"sync"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[chan []byte]struct{}),
	}
}

func (hub *Hub) Subscribe() chan []byte {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	channel := make(chan []byte, 16)
	hub.subscribers[channel] = struct{}{}
	return channel
}

func (hub *Hub) Unsubscribe(channel chan []byte) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if _, ok := hub.subscribers[channel]; ok {
		delete(hub.subscribers, channel)
		close(channel)
	}
}

func (hub *Hub) Publish(event model.EventEnvelope) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	for subscriber := range hub.subscribers {
		select {
		case subscriber <- payload:
		default:
		}
	}
}
