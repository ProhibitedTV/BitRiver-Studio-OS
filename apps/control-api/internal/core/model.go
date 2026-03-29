package core

import "time"

type HealthState string

const (
	HealthHealthy  HealthState = "healthy"
	HealthDegraded HealthState = "degraded"
	HealthError    HealthState = "error"
)

type StreamState string

const (
	StreamStopped  StreamState = "stopped"
	StreamStarting StreamState = "starting"
	StreamLive     StreamState = "live"
)

type RecordState string

const (
	RecordStopped   RecordState = "stopped"
	RecordRecording RecordState = "recording"
)

type SourceStatus string

const (
	SourceReady        SourceStatus = "ready"
	SourceDisconnected SourceStatus = "disconnected"
	SourceWarning      SourceStatus = "warning"
)

type OperationStatus string

const (
	OperationAccepted  OperationStatus = "accepted"
	OperationCompleted OperationStatus = "completed"
	OperationRejected  OperationStatus = "rejected"
)

type HealthCheck struct {
	Key     string      `json:"key"`
	State   HealthState `json:"state"`
	Message string      `json:"message"`
}

type HealthSummary struct {
	Overall HealthState   `json:"overall"`
	Checks  []HealthCheck `json:"checks"`
}

type NetworkSummary struct {
	LANAddress string `json:"lanAddress"`
	Hostname   string `json:"hostname"`
}

type SystemSummary struct {
	ApplianceID   string         `json:"applianceId"`
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	BootMode      string         `json:"bootMode"`
	StartedAt     time.Time      `json:"startedAt"`
	UptimeSeconds int64          `json:"uptimeSeconds"`
	Health        HealthSummary  `json:"health"`
	Network       NetworkSummary `json:"network"`
}

type DestinationSummary struct {
	Mode      string `json:"mode"`
	Label     string `json:"label"`
	Target    string `json:"target"`
	Connected bool   `json:"connected"`
}

type SceneSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ShowState struct {
	ActiveSceneID string             `json:"activeSceneId"`
	StreamState   StreamState        `json:"streamState"`
	RecordState   RecordState        `json:"recordState"`
	Destination   DestinationSummary `json:"destination"`
	Scenes        []SceneSummary     `json:"scenes"`
}

type SourceSummary struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Kind        string       `json:"kind"`
	Status      SourceStatus `json:"status"`
	Detail      string       `json:"detail"`
	BitrateKbps int          `json:"bitrateKbps"`
	PublishURL  string       `json:"publishUrl,omitempty"`
}

type AudioChannelSummary struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Muted  bool    `json:"muted"`
	GainDB float64 `json:"gainDb"`
	PeakDB float64 `json:"peakDb"`
	Source string  `json:"source"`
}

type AudioState struct {
	Channels []AudioChannelSummary `json:"channels"`
}

type StorageSummary struct {
	RecordingPath string `json:"recordingPath"`
	TotalGB       int    `json:"totalGb"`
	UsedGB        int    `json:"usedGb"`
	FreeGB        int    `json:"freeGb"`
	Warning       string `json:"warning,omitempty"`
}

type ConfigSystem struct {
	Name          string `json:"name"`
	Timezone      string `json:"timezone"`
	UpdateChannel string `json:"updateChannel"`
}

type ConfigSource struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	DeviceRef  string   `json:"deviceRef,omitempty"`
	Protocols  []string `json:"protocols,omitempty"`
	PublishURL string   `json:"publishUrl,omitempty"`
}

type ConfigScene struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Sources []string `json:"sources"`
}

type ConfigAudioChannel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	DeviceRef string `json:"deviceRef"`
}

type ConfigAudio struct {
	Channels []ConfigAudioChannel `json:"channels"`
}

type ConfigDestination struct {
	Mode        string `json:"mode"`
	TargetLabel string `json:"targetLabel"`
	TargetURL   string `json:"targetUrl"`
}

type ConfigRecording struct {
	Container string `json:"container"`
	Path      string `json:"path"`
}

type ConfigOwncast struct {
	Enabled   bool   `json:"enabled"`
	PublicURL string `json:"publicUrl,omitempty"`
}

type ControlConfig struct {
	System      ConfigSystem      `json:"system"`
	Sources     []ConfigSource    `json:"sources"`
	Scenes      []ConfigScene     `json:"scenes"`
	Audio       ConfigAudio       `json:"audio"`
	Destination ConfigDestination `json:"destination"`
	Recording   ConfigRecording   `json:"recording"`
	Owncast     ConfigOwncast     `json:"owncast"`
}

type ConfigEnvelope struct {
	Generation string        `json:"generation"`
	UpdatedAt  time.Time     `json:"updatedAt"`
	Config     ControlConfig `json:"config"`
}

type ValidationIssue struct {
	Field    string `json:"field"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ValidateConfigResponse struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues"`
}

type OperationResponse struct {
	OperationID       string          `json:"operationId"`
	Status            OperationStatus `json:"status"`
	Message           string          `json:"message"`
	AppliedGeneration string          `json:"appliedGeneration,omitempty"`
}

type GainRequest struct {
	GainDB float64 `json:"gainDb"`
}
