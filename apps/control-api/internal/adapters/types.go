package adapters

import (
	"context"
	"errors"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

type Runner interface {
	Run(context.Context)
}

type OBSScene struct {
	Name   string
	Index  int
	Active bool
}

type OBSInput struct {
	Name          string
	Kind          string
	HasAudio      bool
	Muted         bool
	GainDB        float64
	PeakDB        float64
	Available     bool
	LastChangedAt *time.Time
}

type OBSVersion struct {
	OBSVersion             string
	WebSocketVersion       string
	Platform               string
	PlatformDescription    string
	RPCVersion             int
	AuthenticationRequired bool
	AvailableRequests      []string
	AvailableEvents        []string
}

type OBSStreamStatus struct {
	Active          bool
	Reconnecting    bool
	Timecode        string
	DurationSeconds int64
	BytesPerSecond  int64
	FramesSkipped   int64
	FramesTotal     int64
	LastChangedAt   *time.Time
}

type OBSRecordStatus struct {
	Active          bool
	Paused          bool
	Path            string
	Filename        string
	Timecode        string
	DurationSeconds int64
	Bytes           int64
	LastChangedAt   *time.Time
}

type OBSEvent struct {
	Type        string
	Timestamp   time.Time
	SceneName   string
	InputName   string
	InputMuted  *bool
	InputGainDB *float64
	Stream      *OBSStreamStatus
	Record      *OBSRecordStatus
}

type OBSSnapshot struct {
	Version         OBSVersion
	Scenes          []OBSScene
	ActiveScene     string
	StreamStatus    OBSStreamStatus
	RecordStatus    OBSRecordStatus
	StreamActive    bool
	RecordingActive bool
	RecordingPath   string
	RecordingFile   string
	Inputs          []OBSInput
	LastChangedAt   *time.Time
}

type OBS interface {
	Snapshot(context.Context) (OBSSnapshot, error)
	Version(context.Context) (OBSVersion, error)
	Scenes(context.Context) ([]OBSScene, error)
	CurrentProgramScene(context.Context) (string, error)
	Inputs(context.Context) ([]OBSInput, error)
	StreamStatus(context.Context) (OBSStreamStatus, error)
	RecordStatus(context.Context) (OBSRecordStatus, error)
	StartStream(context.Context) error
	StopStream(context.Context) error
	StartRecording(context.Context) error
	StopRecording(context.Context) error
	SetCurrentScene(context.Context, string) error
	SetInputMuted(context.Context, string, bool) error
	ToggleInputMuted(context.Context, string) (bool, error)
	SetInputVolume(context.Context, string, float64) error
}

// BrowserSceneInstaller is opt-in: existing OBS control implementations do not
// have to support local scene installation.
type BrowserSceneInstaller interface {
	InstallBrowserScene(context.Context, string, string, string, int, int) error
}

var (
	ErrSceneInstallBusy = errors.New("OBS is streaming or recording")
	ErrSceneInstallCollision = errors.New("OBS scene or input name already exists")
	ErrBrowserSourceUnavailable = errors.New("OBS browser_source input kind is unavailable")
)

type MediaPath struct {
	Name        string
	Source      string
	Readers     int
	Publisher   bool
	Ready       bool
	BitrateKbps int
	PublishURL  string
	Detail      string
}

type MediaSession struct {
	ID         string
	Protocol   string
	Path       string
	RemoteAddr string
	State      string
	StartedAt  *time.Time
}

type MediaDestination struct {
	ID        string
	Name      string
	Protocol  string
	TargetURL string
	Enabled   bool
	Connected bool
	Detail    string
}

type MediaMTXSnapshot struct {
	Ready                 bool
	Healthy               bool
	Detail                string
	ConfigPath            string
	ConfigGeneration      string
	LocalIngestPath       string
	LocalPreviewPath      string
	RecordingPath         string
	WebRTCPreviewEnabled  bool
	Destinations          []MediaDestination
	Paths                 []MediaPath
	Sessions              []MediaSession
	LastAppliedAt         *time.Time
	LastReloadedAt        *time.Time
	LastConfigValidatedAt *time.Time
}

type MediaApplyResult struct {
	ConfigGeneration string
	ConfigPath       string
	ReloadMode       string
	AppliedAt        time.Time
}

type MediaMTX interface {
	Snapshot(context.Context) (MediaMTXSnapshot, error)
	ApplyConfig(context.Context, string, model.ControlConfig) (MediaApplyResult, error)
}

type OwncastStatus struct {
	Connected bool
	Detail    string
}

type Owncast interface {
	Status(context.Context) (OwncastStatus, error)
}

type UnitStatus struct {
	Name       string
	Active     bool
	State      string
	Detail     string
	Restarting bool
}

type Services interface {
	Status(context.Context, []string) ([]UnitStatus, error)
}
