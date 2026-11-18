package model

import "time"

func DefaultConfig() ControlConfig {
	return ControlConfig{
		System: ConfigSystem{
			Name:          "studio-a",
			Timezone:      "America/Denver",
			UpdateChannel: "stable",
		},
		Sources: []ConfigSource{
			{
				ID:        "cam-host",
				Name:      "Host Camera",
				Kind:      "local_video",
				DeviceRef: "usb-046d_0825-hostcam",
			},
			{
				ID:        "capture-game",
				Name:      "Gameplay Capture",
				Kind:      "local_video",
				DeviceRef: "usb-elgato-hd60x-main",
			},
			{
				ID:         "guest-1",
				Name:       "Guest Slot 1",
				Kind:       "network_ingest",
				Protocols:  []string{"rtmp", "srt"},
				PublishURL: "srt://bitriver.local:8890?streamid=publish:guest-1",
			},
			{
				ID:        "lower-third",
				Name:      "Show Overlay",
				Kind:      "browser_source",
				DeviceRef: "https://overlays.bitriver.local/show/lower-third",
			},
		},
		Scenes: []ConfigScene{
			{
				ID:      "starting-soon",
				Name:    "Starting Soon",
				Sources: []string{"lower-third"},
			},
			{
				ID:      "host-cam",
				Name:    "Host Camera",
				Sources: []string{"cam-host", "mic-host", "lower-third"},
			},
			{
				ID:      "split-guest",
				Name:    "Host + Guest",
				Sources: []string{"cam-host", "guest-1", "mic-host", "lower-third"},
			},
		},
		Audio: ConfigAudio{
			Channels: []ConfigAudioChannel{
				{
					ID:        "mic-host",
					Name:      "Host Mic",
					DeviceRef: "usb-focusrite-2i2-input-1",
				},
				{
					ID:        "game-mix",
					Name:      "Game Mix",
					DeviceRef: "usb-elgato-hd60x-audio",
				},
			},
		},
		Destination: ConfigDestination{
			Mode:        "external_rtmp",
			TargetLabel: "Twitch",
			TargetURL:   "rtmp://live.twitch.tv/app",
		},
		Recording: ConfigRecording{
			Container: "mkv",
			Path:      "/data/recordings/program",
		},
		Owncast: ConfigOwncast{
			Enabled:   false,
			PublicURL: "http://bitriver.local:8081",
		},
		MediaRouter: ConfigMediaRouter{
			LocalIngestPath:  "program-live",
			LocalPreviewPath: "program-preview",
			RecordingPath:    "/data/recordings/program",
			PublishDestinations: []ConfigMediaDestination{
				{
					ID:        "primary-rtmp",
					Name:      "Twitch",
					Protocol:  "rtmp",
					TargetURL: "rtmp://live.twitch.tv/app",
					Enabled:   true,
				},
			},
			WebRTCPreviewEnabled: false,
		},
	}
}

func DefaultHardwareReport(now time.Time) HardwareReport {
	return HardwareReport{
		DetectedAt:          now.UTC(),
		Architecture:        "x86_64",
		CPUModel:            "unknown",
		LogicalCores:        4,
		MemoryMiB:           16384,
		DisplayAttached:     false,
		BootFirmware:        "unknown",
		GPUs:                []GPUDevice{},
		NetworkAdapters:     []NetworkAdapter{},
		CompatibilityNotes:  []string{"No hardware validation has been performed yet."},
		VideoDevices:        []string{},
		AudioCaptureDevices: []string{},
		NetworkInterfaces:   []string{"eth0"},
		Capabilities: HardwareCapabilities{
			KioskEligible:     false,
			OBSEligible:       false,
			RecommendedForMVP: false,
		},
	}
}

func DefaultSetupStatus(now time.Time) SetupStatus {
	hardware := DefaultHardwareReport(now)
	return SetupStatus{
		State:                    "pending",
		FirstBootComplete:        false,
		ApplianceID:              "studio-appliance-dev",
		Hostname:                 "bitriver.local",
		AdminUsername:            "admin",
		CredentialsReady:         false,
		RequiresPasswordRotation: true,
		InitialConfigWritten:     false,
		HardwareDetected:         false,
		DisplayMode:              "headless",
		StartedAt:                now.UTC(),
		Issues:                   []string{"first-boot setup has not completed"},
		Hardware:                 hardware,
	}
}

func DefaultUpdateStatus(now time.Time, version string) UpdateStatus {
	return UpdateStatus{
		State:            "idle",
		CurrentVersion:   version,
		DeploymentSource: "fallback",
		LastCheckedAt:    pointerToTime(now.UTC()),
		Message:          "update agent has not published status yet",
	}
}

func EnvelopeFromDocument(document ConfigDocument) ConfigEnvelope {
	return ConfigEnvelope{
		SchemaVersion: document.SchemaVersion,
		Generation:    document.Generation,
		UpdatedAt:     document.UpdatedAt.UTC(),
		Config:        CloneConfig(document.ControlConfig),
	}
}

func NewDocument(generation string, updatedAt time.Time, config ControlConfig) ConfigDocument {
	return ConfigDocument{
		SchemaVersion: CurrentSchemaVersion,
		Generation:    generation,
		UpdatedAt:     updatedAt.UTC(),
		ControlConfig: CloneConfig(NormalizeConfig(config)),
	}
}

func CloneConfig(in ControlConfig) ControlConfig {
	out := in
	out.Sources = append([]ConfigSource(nil), in.Sources...)
	out.Scenes = make([]ConfigScene, len(in.Scenes))
	for index, scene := range in.Scenes {
		copied := scene
		copied.Sources = append([]string(nil), scene.Sources...)
		out.Scenes[index] = copied
	}
	out.Audio.Channels = append([]ConfigAudioChannel(nil), in.Audio.Channels...)
	out.MediaRouter.PublishDestinations = append([]ConfigMediaDestination(nil), in.MediaRouter.PublishDestinations...)
	return out
}

func NormalizeConfig(in ControlConfig) ControlConfig {
	defaults := DefaultConfig()
	out := in

	if out.System.Name == "" {
		out.System.Name = defaults.System.Name
	}
	if out.System.Timezone == "" {
		out.System.Timezone = defaults.System.Timezone
	}
	if out.System.UpdateChannel == "" {
		out.System.UpdateChannel = defaults.System.UpdateChannel
	}

	if out.Destination.Mode == "" {
		out.Destination.Mode = defaults.Destination.Mode
	}
	if out.Destination.TargetLabel == "" {
		out.Destination.TargetLabel = defaults.Destination.TargetLabel
	}
	if out.Destination.TargetURL == "" {
		out.Destination.TargetURL = defaults.Destination.TargetURL
	}

	if out.Recording.Container == "" {
		out.Recording.Container = defaults.Recording.Container
	}

	if out.Owncast.PublicURL == "" {
		out.Owncast.PublicURL = defaults.Owncast.PublicURL
	}

	if out.MediaRouter.LocalIngestPath == "" {
		out.MediaRouter.LocalIngestPath = defaults.MediaRouter.LocalIngestPath
	}
	if out.MediaRouter.LocalPreviewPath == "" {
		out.MediaRouter.LocalPreviewPath = defaults.MediaRouter.LocalPreviewPath
	}
	if out.MediaRouter.RecordingPath == "" {
		if out.Recording.Path != "" {
			out.MediaRouter.RecordingPath = out.Recording.Path
		} else {
			out.MediaRouter.RecordingPath = defaults.MediaRouter.RecordingPath
		}
	}
	if len(out.MediaRouter.PublishDestinations) == 0 {
		if out.Destination.TargetURL != "" {
			out.MediaRouter.PublishDestinations = []ConfigMediaDestination{
				{
					ID:        "primary-" + destinationProtocol(out.Destination.Mode),
					Name:      fallbackString(out.Destination.TargetLabel, "Primary Destination"),
					Protocol:  destinationProtocol(out.Destination.Mode),
					TargetURL: out.Destination.TargetURL,
					Enabled:   true,
				},
			}
		} else {
			out.MediaRouter.PublishDestinations = append([]ConfigMediaDestination(nil), defaults.MediaRouter.PublishDestinations...)
		}
	}
	if out.Recording.Path == "" {
		out.Recording.Path = out.MediaRouter.RecordingPath
	}

	primaryDestination := firstEnabledDestination(out.MediaRouter.PublishDestinations)
	if primaryDestination != nil {
		if out.Destination.Mode == "" || out.Destination.Mode == defaults.Destination.Mode || out.Destination.Mode == "external_srt" {
			out.Destination.Mode = protocolToDestinationMode(primaryDestination.Protocol)
		}
		if out.Destination.TargetLabel == "" || out.Destination.TargetLabel == defaults.Destination.TargetLabel {
			out.Destination.TargetLabel = primaryDestination.Name
		}
		if out.Destination.TargetURL == "" {
			out.Destination.TargetURL = primaryDestination.TargetURL
		}
	}

	return CloneConfig(out)
}

func firstEnabledDestination(destinations []ConfigMediaDestination) *ConfigMediaDestination {
	for index := range destinations {
		if destinations[index].Enabled {
			return &destinations[index]
		}
	}
	if len(destinations) == 0 {
		return nil
	}
	return &destinations[0]
}

func destinationProtocol(mode string) string {
	switch mode {
	case "external_srt":
		return "srt"
	default:
		return "rtmp"
	}
}

func protocolToDestinationMode(protocol string) string {
	switch protocol {
	case "srt":
		return "external_srt"
	default:
		return "external_rtmp"
	}
}

func fallbackString(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func CloneStrings(in []string) []string {
	return append([]string(nil), in...)
}

func CloneHealthChecks(in []HealthCheck) []HealthCheck {
	return append([]HealthCheck(nil), in...)
}

func CloneScenes(in []SceneSummary) []SceneSummary {
	return append([]SceneSummary(nil), in...)
}

func CloneSources(in []SourceSummary) []SourceSummary {
	return append([]SourceSummary(nil), in...)
}

func CloneAudioChannels(in []AudioChannelSummary) []AudioChannelSummary {
	return append([]AudioChannelSummary(nil), in...)
}

func CloneSetupStatus(in SetupStatus) SetupStatus {
	out := in
	out.Issues = CloneStrings(in.Issues)
	out.Hardware.VideoDevices = CloneStrings(in.Hardware.VideoDevices)
	out.Hardware.AudioCaptureDevices = CloneStrings(in.Hardware.AudioCaptureDevices)
	out.Hardware.NetworkInterfaces = CloneStrings(in.Hardware.NetworkInterfaces)
	out.Hardware.GPUs = append([]GPUDevice(nil), in.Hardware.GPUs...)
	out.Hardware.NetworkAdapters = append([]NetworkAdapter(nil), in.Hardware.NetworkAdapters...)
	out.Hardware.CompatibilityNotes = CloneStrings(in.Hardware.CompatibilityNotes)
	return out
}

func pointerToTime(value time.Time) *time.Time {
	return &value
}
