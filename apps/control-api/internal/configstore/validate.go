package configstore

import (
	"strings"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

func ValidateConfig(config model.ControlConfig) model.ValidateConfigResponse {
	config = model.NormalizeConfig(config)
	issues := make([]model.ValidationIssue, 0)

	if config.System.Name == "" {
		issues = append(issues, model.ValidationIssue{
			Field:    "system.name",
			Severity: "error",
			Message:  "system name is required",
		})
	}

	if config.Recording.Path == "" {
		issues = append(issues, model.ValidationIssue{
			Field:    "recording.path",
			Severity: "error",
			Message:  "recording path is required",
		})
	}

	if len(config.Scenes) == 0 {
		issues = append(issues, model.ValidationIssue{
			Field:    "scenes",
			Severity: "error",
			Message:  "at least one scene is required",
		})
	}

	if config.Destination.Mode != "external_rtmp" && config.Destination.Mode != "external_srt" && config.Destination.Mode != "owncast" {
		issues = append(issues, model.ValidationIssue{
			Field:    "destination.mode",
			Severity: "error",
			Message:  "destination mode must be external_rtmp, external_srt, or owncast",
		})
	}

	if config.MediaRouter.LocalIngestPath == "" {
		issues = append(issues, model.ValidationIssue{
			Field:    "mediaRouter.localIngestPath",
			Severity: "error",
			Message:  "media router local ingest path is required",
		})
	}

	if config.MediaRouter.LocalPreviewPath == "" {
		issues = append(issues, model.ValidationIssue{
			Field:    "mediaRouter.localPreviewPath",
			Severity: "error",
			Message:  "media router local preview path is required",
		})
	}

	if config.MediaRouter.LocalIngestPath != "" && config.MediaRouter.LocalIngestPath == config.MediaRouter.LocalPreviewPath {
		issues = append(issues, model.ValidationIssue{
			Field:    "mediaRouter.localPreviewPath",
			Severity: "error",
			Message:  "media router preview path must differ from the ingest path",
		})
	}

	if config.MediaRouter.RecordingPath == "" {
		issues = append(issues, model.ValidationIssue{
			Field:    "mediaRouter.recordingPath",
			Severity: "error",
			Message:  "media router recording path is required",
		})
	}

	if !strings.HasPrefix(config.MediaRouter.RecordingPath, "/") {
		issues = append(issues, model.ValidationIssue{
			Field:    "mediaRouter.recordingPath",
			Severity: "error",
			Message:  "media router recording path must be an absolute path",
		})
	}

	if len(config.MediaRouter.PublishDestinations) == 0 {
		issues = append(issues, model.ValidationIssue{
			Field:    "mediaRouter.publishDestinations",
			Severity: "warning",
			Message:  "no downstream publish destinations are configured; the appliance will preview and record locally only",
		})
	}

	seenDestinationIDs := make(map[string]struct{}, len(config.MediaRouter.PublishDestinations))
	for _, destination := range config.MediaRouter.PublishDestinations {
		if destination.ID == "" {
			issues = append(issues, model.ValidationIssue{
				Field:    "mediaRouter.publishDestinations.id",
				Severity: "error",
				Message:  "media destination id is required",
			})
		}
		if destination.ID != "" {
			if _, exists := seenDestinationIDs[destination.ID]; exists {
				issues = append(issues, model.ValidationIssue{
					Field:    "mediaRouter.publishDestinations",
					Severity: "error",
					Message:  "duplicate media destination id: " + destination.ID,
				})
			}
			seenDestinationIDs[destination.ID] = struct{}{}
		}

		if destination.Name == "" {
			issues = append(issues, model.ValidationIssue{
				Field:    "mediaRouter.publishDestinations.name",
				Severity: "error",
				Message:  "media destination name is required",
			})
		}

		if err := model.ValidateStreamKeyFile(destination); err != nil {
			issues = append(issues, model.ValidationIssue{
				Field:    "mediaRouter.publishDestinations.streamKeyFile",
				Severity: "error",
				Message:  err.Error(),
			})
		}
		switch destination.Protocol {
		case "rtmp":
			if !strings.HasPrefix(destination.TargetURL, "rtmp://") && !strings.HasPrefix(destination.TargetURL, "rtmps://") {
				issues = append(issues, model.ValidationIssue{
					Field:    "mediaRouter.publishDestinations.targetUrl",
					Severity: "error",
					Message:  "RTMP destinations must use rtmp:// or rtmps:// URLs",
				})
			}
		case "srt":
			if !strings.HasPrefix(destination.TargetURL, "srt://") {
				issues = append(issues, model.ValidationIssue{
					Field:    "mediaRouter.publishDestinations.targetUrl",
					Severity: "error",
					Message:  "SRT destinations must use srt:// URLs",
				})
			}
		default:
			issues = append(issues, model.ValidationIssue{
				Field:    "mediaRouter.publishDestinations.protocol",
				Severity: "error",
				Message:  "media destination protocol must be rtmp or srt",
			})
		}
	}

	knownRefs := make(map[string]struct{}, len(config.Sources)+len(config.Audio.Channels))
	seenSourceIDs := make(map[string]struct{}, len(config.Sources))
	for _, source := range config.Sources {
		if source.ID == "" {
			issues = append(issues, model.ValidationIssue{
				Field:    "sources.id",
				Severity: "error",
				Message:  "source id is required",
			})
			continue
		}
		if _, exists := seenSourceIDs[source.ID]; exists {
			issues = append(issues, model.ValidationIssue{
				Field:    "sources",
				Severity: "error",
				Message:  "duplicate source id: " + source.ID,
			})
			continue
		}
		seenSourceIDs[source.ID] = struct{}{}
		knownRefs[source.ID] = struct{}{}
	}

	seenAudioIDs := make(map[string]struct{}, len(config.Audio.Channels))
	for _, channel := range config.Audio.Channels {
		if channel.ID == "" {
			issues = append(issues, model.ValidationIssue{
				Field:    "audio.channels.id",
				Severity: "error",
				Message:  "audio channel id is required",
			})
			continue
		}
		if _, exists := seenAudioIDs[channel.ID]; exists {
			issues = append(issues, model.ValidationIssue{
				Field:    "audio.channels",
				Severity: "error",
				Message:  "duplicate audio channel id: " + channel.ID,
			})
			continue
		}
		seenAudioIDs[channel.ID] = struct{}{}
		knownRefs[channel.ID] = struct{}{}
	}

	seenSceneIDs := make(map[string]struct{}, len(config.Scenes))
	for _, scene := range config.Scenes {
		if scene.ID == "" {
			issues = append(issues, model.ValidationIssue{
				Field:    "scenes.id",
				Severity: "error",
				Message:  "scene id is required",
			})
			continue
		}
		if _, exists := seenSceneIDs[scene.ID]; exists {
			issues = append(issues, model.ValidationIssue{
				Field:    "scenes",
				Severity: "error",
				Message:  "duplicate scene id: " + scene.ID,
			})
		}
		seenSceneIDs[scene.ID] = struct{}{}

		for _, ref := range scene.Sources {
			if _, exists := knownRefs[ref]; !exists {
				issues = append(issues, model.ValidationIssue{
					Field:    "scenes",
					Severity: "error",
					Message:  "scene " + scene.ID + " references unknown source or audio id: " + ref,
				})
			}
		}
	}

	return model.ValidateConfigResponse{
		Valid:  len(issues) == 0,
		Issues: issues,
	}
}
