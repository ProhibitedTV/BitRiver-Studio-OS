package core

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
			Path:      "/data/recordings",
		},
		Owncast: ConfigOwncast{
			Enabled:   false,
			PublicURL: "http://bitriver.local:8081",
		},
	}
}

func newEnvelope(generation string, updatedAt time.Time, config ControlConfig) ConfigEnvelope {
	return ConfigEnvelope{
		Generation: generation,
		UpdatedAt:  updatedAt.UTC(),
		Config:     cloneConfig(config),
	}
}

func cloneConfig(in ControlConfig) ControlConfig {
	out := in
	out.Sources = append([]ConfigSource(nil), in.Sources...)
	out.Scenes = make([]ConfigScene, len(in.Scenes))
	for index, scene := range in.Scenes {
		copied := scene
		copied.Sources = append([]string(nil), scene.Sources...)
		out.Scenes[index] = copied
	}
	out.Audio.Channels = append([]ConfigAudioChannel(nil), in.Audio.Channels...)
	return out
}

func cloneSources(in []SourceSummary) []SourceSummary {
	return append([]SourceSummary(nil), in...)
}

func cloneScenes(in []SceneSummary) []SceneSummary {
	return append([]SceneSummary(nil), in...)
}

func cloneAudioChannels(in []AudioChannelSummary) []AudioChannelSummary {
	return append([]AudioChannelSummary(nil), in...)
}
