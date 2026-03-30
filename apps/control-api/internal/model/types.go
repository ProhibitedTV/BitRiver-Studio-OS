package model

import "time"

const CurrentSchemaVersion = "bitriver.control.settings/v1alpha2"

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
	Overall   HealthState   `json:"overall"`
	Checks    []HealthCheck `json:"checks"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type ServiceStatus struct {
	Name       string `json:"name"`
	Active     bool   `json:"active"`
	State      string `json:"state"`
	Detail     string `json:"detail"`
	ManagedBy  string `json:"managedBy"`
	Restarting bool   `json:"restarting"`
}

type NetworkSummary struct {
	LANAddress string   `json:"lanAddress"`
	Hostname   string   `json:"hostname"`
	Interfaces []string `json:"interfaces"`
}

type CPUStatus struct {
	Model        string  `json:"model"`
	LogicalCores int     `json:"logicalCores"`
	UsagePercent float64 `json:"usagePercent"`
	LoadAverage  float64 `json:"loadAverage"`
}

type MemoryStatus struct {
	TotalMiB int `json:"totalMiB"`
	UsedMiB  int `json:"usedMiB"`
	FreeMiB  int `json:"freeMiB"`
}

type DiskStatus struct {
	Path      string `json:"path"`
	TotalGb   int    `json:"totalGb"`
	UsedGb    int    `json:"usedGb"`
	FreeGb    int    `json:"freeGb"`
	Warning   string `json:"warning,omitempty"`
	Available bool   `json:"available"`
}

type SystemSummary struct {
	ApplianceID   string          `json:"applianceId"`
	Name          string          `json:"name"`
	Version       string          `json:"version"`
	BootMode      string          `json:"bootMode"`
	StartedAt     time.Time       `json:"startedAt"`
	UptimeSeconds int64           `json:"uptimeSeconds"`
	Health        HealthSummary   `json:"health"`
	Network       NetworkSummary  `json:"network"`
	CPU           CPUStatus       `json:"cpu"`
	Memory        MemoryStatus    `json:"memory"`
	Disk          DiskStatus      `json:"disk"`
	Services      []ServiceStatus `json:"services"`
}

type DestinationSummary struct {
	ID        string `json:"id"`
	Mode      string `json:"mode"`
	Label     string `json:"label"`
	Target    string `json:"target"`
	Connected bool   `json:"connected"`
	Enabled   bool   `json:"enabled"`
	Detail    string `json:"detail"`
}

type OutputSummary struct {
	StreamState  StreamState          `json:"streamState"`
	Destinations []DestinationSummary `json:"destinations"`
}

type SceneSummary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type ScenesState struct {
	ActiveSceneID string         `json:"activeSceneId"`
	Items         []SceneSummary `json:"items"`
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
	Available   bool         `json:"available"`
	Detail      string       `json:"detail"`
	BitrateKbps int          `json:"bitrateKbps"`
	PublishURL  string       `json:"publishUrl,omitempty"`
}

type AudioChannelSummary struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Muted     bool    `json:"muted"`
	GainDB    float64 `json:"gainDb"`
	PeakDB    float64 `json:"peakDb"`
	Source    string  `json:"source"`
	Available bool    `json:"available"`
}

type AudioState struct {
	Channels []AudioChannelSummary `json:"channels"`
}

type RecordingSummary struct {
	State         RecordState `json:"state"`
	RecordingPath string      `json:"recordingPath"`
	CurrentFile   string      `json:"currentFile,omitempty"`
	TotalGb       int         `json:"totalGb"`
	UsedGb        int         `json:"usedGb"`
	FreeGb        int         `json:"freeGb"`
	Warning       string      `json:"warning,omitempty"`
}

type StorageSummary = RecordingSummary

type MediaDestinationSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Target    string `json:"target"`
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Detail    string `json:"detail"`
}

type MediaPathSummary struct {
	Name        string `json:"name"`
	Ready       bool   `json:"ready"`
	Source      string `json:"source"`
	Readers     int    `json:"readers"`
	BitrateKbps int    `json:"bitrateKbps"`
	Detail      string `json:"detail"`
	PublishURL  string `json:"publishUrl,omitempty"`
}

type MediaSessionSummary struct {
	ID         string     `json:"id"`
	Protocol   string     `json:"protocol"`
	Path       string     `json:"path"`
	RemoteAddr string     `json:"remoteAddr"`
	State      string     `json:"state"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
}

type MediaRouterSummary struct {
	Healthy               bool                      `json:"healthy"`
	Detail                string                    `json:"detail"`
	ConfigGeneration      string                    `json:"configGeneration,omitempty"`
	ConfigPath            string                    `json:"configPath,omitempty"`
	LocalIngestPath       string                    `json:"localIngestPath"`
	LocalPreviewPath      string                    `json:"localPreviewPath"`
	RecordingPath         string                    `json:"recordingPath"`
	WebRTCPreviewEnabled  bool                      `json:"webrtcPreviewEnabled"`
	Destinations          []MediaDestinationSummary `json:"destinations"`
	ActivePaths           []MediaPathSummary        `json:"activePaths"`
	ActiveSessions        []MediaSessionSummary     `json:"activeSessions"`
	LastAppliedAt         *time.Time                `json:"lastAppliedAt,omitempty"`
	LastReloadedAt        *time.Time                `json:"lastReloadedAt,omitempty"`
	LastConfigValidatedAt *time.Time                `json:"lastConfigValidatedAt,omitempty"`
}

type HardwareCapabilities struct {
	KioskEligible     bool `json:"kioskEligible"`
	OBSEligible       bool `json:"obsEligible"`
	RecommendedForMVP bool `json:"recommendedForMvp"`
}

type HardwareReport struct {
	DetectedAt          time.Time            `json:"detectedAt"`
	Architecture        string               `json:"architecture"`
	CPUModel            string               `json:"cpuModel"`
	LogicalCores        int                  `json:"logicalCores"`
	MemoryMiB           int                  `json:"memoryMiB"`
	DisplayAttached     bool                 `json:"displayAttached"`
	VideoDevices        []string             `json:"videoDevices"`
	AudioCaptureDevices []string             `json:"audioCaptureDevices"`
	NetworkInterfaces   []string             `json:"networkInterfaces"`
	Capabilities        HardwareCapabilities `json:"capabilities"`
}

type SetupStatus struct {
	State                    string         `json:"state"`
	FirstBootComplete        bool           `json:"firstBootComplete"`
	ApplianceID              string         `json:"applianceId"`
	Hostname                 string         `json:"hostname"`
	AdminUsername            string         `json:"adminUsername"`
	CredentialsReady         bool           `json:"credentialsReady"`
	RequiresPasswordRotation bool           `json:"requiresPasswordRotation"`
	InitialConfigWritten     bool           `json:"initialConfigWritten"`
	HardwareDetected         bool           `json:"hardwareDetected"`
	DisplayMode              string         `json:"displayMode"`
	StartedAt                time.Time      `json:"startedAt"`
	CompletedAt              *time.Time     `json:"completedAt,omitempty"`
	Issues                   []string       `json:"issues"`
	Hardware                 HardwareReport `json:"hardware"`
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

type ConfigMediaDestination struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	TargetURL string `json:"targetUrl"`
	Enabled   bool   `json:"enabled"`
}

type ConfigMediaRouter struct {
	LocalIngestPath      string                   `json:"localIngestPath"`
	LocalPreviewPath     string                   `json:"localPreviewPath"`
	RecordingPath        string                   `json:"recordingPath"`
	PublishDestinations  []ConfigMediaDestination `json:"publishDestinations"`
	WebRTCPreviewEnabled bool                     `json:"webrtcPreviewEnabled"`
}

type ControlConfig struct {
	System      ConfigSystem      `json:"system"`
	Sources     []ConfigSource    `json:"sources"`
	Scenes      []ConfigScene     `json:"scenes"`
	Audio       ConfigAudio       `json:"audio"`
	Destination ConfigDestination `json:"destination"`
	Recording   ConfigRecording   `json:"recording"`
	Owncast     ConfigOwncast     `json:"owncast"`
	MediaRouter ConfigMediaRouter `json:"mediaRouter"`
}

type ConfigDocument struct {
	SchemaVersion string    `json:"schemaVersion"`
	Generation    string    `json:"generation"`
	UpdatedAt     time.Time `json:"updatedAt"`
	ControlConfig
}

type ConfigEnvelope struct {
	SchemaVersion string        `json:"schemaVersion"`
	Generation    string        `json:"generation"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	Config        ControlConfig `json:"config"`
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

type UpdateStatus struct {
	State            string     `json:"state"`
	CurrentVersion   string     `json:"currentVersion"`
	StagedVersion    string     `json:"stagedVersion,omitempty"`
	DeploymentSource string     `json:"deploymentSource"`
	RebootRequired   bool       `json:"rebootRequired"`
	LastCheckedAt    *time.Time `json:"lastCheckedAt,omitempty"`
	Message          string     `json:"message,omitempty"`
}

type StreamSummary struct {
	State         StreamState `json:"state"`
	ActiveSceneID string      `json:"activeSceneId"`
	Destination   string      `json:"destination"`
	StartedAt     *time.Time  `json:"startedAt,omitempty"`
}

type EventEnvelope struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

type Snapshot struct {
	System      SystemSummary      `json:"system"`
	Health      HealthSummary      `json:"health"`
	Stream      StreamSummary      `json:"stream"`
	Outputs     OutputSummary      `json:"outputs"`
	Recordings  RecordingSummary   `json:"recordings"`
	MediaRouter MediaRouterSummary `json:"mediaRouter"`
	Scenes      ScenesState        `json:"scenes"`
	Sources     []SourceSummary    `json:"sources"`
	Audio       AudioState         `json:"audio"`
	Settings    ConfigEnvelope     `json:"settings"`
	Update      UpdateStatus       `json:"update"`
	Setup       SetupStatus        `json:"setup"`
}
