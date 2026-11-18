package mediamtx

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

var pathNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type renderedConfig struct {
	configYAML       []byte
	destinationsJSON []byte
	snapshot         adapters.MediaMTXSnapshot
}

type destinationsDocument struct {
	ConfigGeneration    string                         `json:"configGeneration"`
	GeneratedAt         time.Time                      `json:"generatedAt"`
	LocalIngestPath     string                         `json:"localIngestPath"`
	LocalPreviewPath    string                         `json:"localPreviewPath"`
	RecordingPath       string                         `json:"recordingPath"`
	WebRTCPreview       bool                           `json:"webrtcPreviewEnabled"`
	PublishDestinations []model.ConfigMediaDestination `json:"publishDestinations"`
}

func renderManagedConfig(generation string, config model.ControlConfig, configPath string, destinationsPath string) (renderedConfig, error) {
	config = model.NormalizeConfig(config)
	if err := validateManagedConfig(config); err != nil {
		return renderedConfig{}, err
	}

	now := time.Now().UTC()
	recordPattern := path.Join(cleanLinuxPath(config.MediaRouter.RecordingPath), "%path", "%Y-%m-%d_%H-%M-%S-%f")
	rtspSource := fmt.Sprintf("rtsp://127.0.0.1:8554/%s", config.MediaRouter.LocalIngestPath)

	var builder strings.Builder
	writeLine(&builder, "logLevel: info")
	writeLine(&builder, "api: yes")
	writeLine(&builder, "apiAddress: 127.0.0.1:9997")
	writeLine(&builder, "rtmp: yes")
	writeLine(&builder, "rtmpAddress: :1935")
	writeLine(&builder, "rtsp: yes")
	writeLine(&builder, "rtspAddress: :8554")
	writeLine(&builder, "srt: yes")
	writeLine(&builder, "srtAddress: :8890")
	if config.MediaRouter.WebRTCPreviewEnabled {
		writeLine(&builder, "webrtc: yes")
		writeLine(&builder, "webrtcAddress: :8889")
	} else {
		writeLine(&builder, "webrtc: no")
	}
	writeLine(&builder, "")
	writeLine(&builder, "paths:")
	writeLine(&builder, "  "+config.MediaRouter.LocalIngestPath+":")
	writeLine(&builder, "    source: publisher")
	writeLine(&builder, "    record: yes")
	writeLine(&builder, "    recordPath: "+yamlScalar(recordPattern))
	if hasEnabledDestinations(config.MediaRouter.PublishDestinations) {
		runOnReady := fmt.Sprintf(
			"/usr/local/libexec/bitriver/mediamtx-restream.sh %s %s",
			shellQuote(destinationsPath),
			shellQuote(rtspSource),
		)
		writeLine(&builder, "    runOnReady: "+yamlScalar(runOnReady))
		writeLine(&builder, "    runOnReadyRestart: yes")
	}

	writeLine(&builder, "  "+config.MediaRouter.LocalPreviewPath+":")
	writeLine(&builder, "    source: "+yamlScalar(rtspSource))

	networkPaths := collectNetworkPaths(config.Sources)
	for _, pathName := range networkPaths {
		writeLine(&builder, "  "+pathName+":")
		writeLine(&builder, "    source: publisher")
	}

	destinations := make([]model.ConfigMediaDestination, 0, len(config.MediaRouter.PublishDestinations))
	snapshotDestinations := make([]adapters.MediaDestination, 0, len(config.MediaRouter.PublishDestinations))
	for _, destination := range config.MediaRouter.PublishDestinations {
		destinations = append(destinations, destination)
		snapshotDestinations = append(snapshotDestinations, adapters.MediaDestination{
			ID:        destination.ID,
			Name:      destination.Name,
			Protocol:  destination.Protocol,
			TargetURL: destination.TargetURL,
			Enabled:   destination.Enabled,
			Connected: false,
			Detail:    destinationDetail(destination.Enabled, false),
		})
	}

	destinationsPayload, err := json.MarshalIndent(destinationsDocument{
		ConfigGeneration:    generation,
		GeneratedAt:         now,
		LocalIngestPath:     config.MediaRouter.LocalIngestPath,
		LocalPreviewPath:    config.MediaRouter.LocalPreviewPath,
		RecordingPath:       config.MediaRouter.RecordingPath,
		WebRTCPreview:       config.MediaRouter.WebRTCPreviewEnabled,
		PublishDestinations: destinations,
	}, "", "  ")
	if err != nil {
		return renderedConfig{}, fmt.Errorf("marshal destinations sidecar: %w", err)
	}

	return renderedConfig{
		configYAML:       append([]byte(builder.String()), '\n'),
		destinationsJSON: append(destinationsPayload, '\n'),
		snapshot: adapters.MediaMTXSnapshot{
			Ready:                 false,
			Healthy:               false,
			Detail:                "MediaMTX configuration staged",
			ConfigPath:            configPath,
			ConfigGeneration:      generation,
			LocalIngestPath:       config.MediaRouter.LocalIngestPath,
			LocalPreviewPath:      config.MediaRouter.LocalPreviewPath,
			RecordingPath:         config.MediaRouter.RecordingPath,
			WebRTCPreviewEnabled:  config.MediaRouter.WebRTCPreviewEnabled,
			Destinations:          snapshotDestinations,
			Paths:                 renderedPaths(config, networkPaths),
			Sessions:              []adapters.MediaSession{},
			LastConfigValidatedAt: &now,
		},
	}, nil
}

func validateManagedConfig(config model.ControlConfig) error {
	config = model.NormalizeConfig(config)
	usedPaths := map[string]string{}

	for field, value := range map[string]string{
		"mediaRouter.localIngestPath":  config.MediaRouter.LocalIngestPath,
		"mediaRouter.localPreviewPath": config.MediaRouter.LocalPreviewPath,
	} {
		if value == "" {
			return fmt.Errorf("%s is required", field)
		}
		if !pathNamePattern.MatchString(value) {
			return fmt.Errorf("%s must contain only letters, numbers, dots, dashes, and underscores", field)
		}
		if existingField, exists := usedPaths[value]; exists {
			return fmt.Errorf("%s collides with %s", field, existingField)
		}
		usedPaths[value] = field
	}

	if config.MediaRouter.RecordingPath == "" {
		return fmt.Errorf("mediaRouter.recordingPath is required")
	}
	if !strings.HasPrefix(config.MediaRouter.RecordingPath, "/") {
		return fmt.Errorf("mediaRouter.recordingPath must be an absolute path")
	}

	for _, source := range config.Sources {
		if source.Kind != "network_ingest" {
			continue
		}
		if source.ID == "" {
			return fmt.Errorf("network ingest sources require a non-empty id")
		}
		if !pathNamePattern.MatchString(source.ID) {
			return fmt.Errorf("source %q uses an invalid network ingest path", source.ID)
		}
		if existingField, exists := usedPaths[source.ID]; exists {
			return fmt.Errorf("network ingest path %q collides with %s", source.ID, existingField)
		}
		usedPaths[source.ID] = "sources." + source.ID
	}

	for _, destination := range config.MediaRouter.PublishDestinations {
		if destination.ID == "" {
			return fmt.Errorf("mediaRouter.publishDestinations.id is required")
		}
		if destination.Name == "" {
			return fmt.Errorf("mediaRouter.publishDestinations.name is required")
		}
		if err := model.ValidateStreamKeyFile(destination); err != nil {
			return fmt.Errorf("destination %q credential reference: %w", destination.ID, err)
		}
		switch destination.Protocol {
		case "rtmp":
			if !strings.HasPrefix(destination.TargetURL, "rtmp://") && !strings.HasPrefix(destination.TargetURL, "rtmps://") {
				return fmt.Errorf("RTMP destination %q must use an rtmp:// or rtmps:// URL", destination.ID)
			}
		case "srt":
			if !strings.HasPrefix(destination.TargetURL, "srt://") {
				return fmt.Errorf("SRT destination %q must use an srt:// URL", destination.ID)
			}
		default:
			return fmt.Errorf("destination %q must use the rtmp or srt protocol", destination.ID)
		}
	}

	return nil
}

func collectNetworkPaths(sources []model.ConfigSource) []string {
	paths := make([]string, 0, len(sources))
	for _, source := range sources {
		if source.Kind == "network_ingest" && source.ID != "" {
			paths = append(paths, source.ID)
		}
	}
	sort.Strings(paths)
	return paths
}

func renderedPaths(config model.ControlConfig, networkPaths []string) []adapters.MediaPath {
	paths := []adapters.MediaPath{
		{
			Name:        config.MediaRouter.LocalIngestPath,
			Source:      "publisher",
			Readers:     1,
			Publisher:   false,
			Ready:       false,
			BitrateKbps: 0,
			PublishURL:  "rtmp://127.0.0.1:1935/" + config.MediaRouter.LocalIngestPath,
			Detail:      "Awaiting OBS publish",
		},
		{
			Name:        config.MediaRouter.LocalPreviewPath,
			Source:      "rtsp://127.0.0.1:8554/" + config.MediaRouter.LocalIngestPath,
			Readers:     0,
			Publisher:   false,
			Ready:       false,
			BitrateKbps: 0,
			Detail:      previewDetail(config.MediaRouter.WebRTCPreviewEnabled, false),
		},
	}

	for _, pathName := range networkPaths {
		paths = append(paths, adapters.MediaPath{
			Name:        pathName,
			Source:      "publisher",
			Readers:     0,
			Publisher:   false,
			Ready:       false,
			BitrateKbps: 0,
			PublishURL:  "srt://bitriver.local:8890?streamid=publish:" + pathName,
			Detail:      "Awaiting contributor publish",
		})
	}

	return paths
}

func hasEnabledDestinations(destinations []model.ConfigMediaDestination) bool {
	for _, destination := range destinations {
		if destination.Enabled {
			return true
		}
	}
	return false
}

func destinationDetail(enabled bool, connected bool) string {
	if !enabled {
		return "Destination is disabled"
	}
	if connected {
		return "Media router is fanning out program output"
	}
	return "Waiting for local program ingest"
}

func previewDetail(webrtcEnabled bool, ready bool) string {
	if ready {
		if webrtcEnabled {
			return "Preview path is ready for RTSP and WebRTC readers"
		}
		return "Preview path is ready for RTSP readers"
	}
	if webrtcEnabled {
		return "Preview path is idle; WebRTC preview will come online when OBS publishes"
	}
	return "Preview path is idle"
}

func yamlScalar(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func writeLine(builder *strings.Builder, value string) {
	builder.WriteString(value)
	builder.WriteByte('\n')
}

func cleanLinuxPath(value string) string {
	cleaned := path.Clean("/" + strings.TrimPrefix(value, "/"))
	return strings.TrimSuffix(cleaned, "/")
}
