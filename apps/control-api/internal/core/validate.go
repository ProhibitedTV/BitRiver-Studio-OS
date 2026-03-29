package core

func ValidateConfig(config ControlConfig) ValidateConfigResponse {
	issues := make([]ValidationIssue, 0)

	if len(config.Scenes) == 0 {
		issues = append(issues, ValidationIssue{
			Field:    "scenes",
			Severity: "error",
			Message:  "at least one scene is required",
		})
	}

	if config.Recording.Path == "" {
		issues = append(issues, ValidationIssue{
			Field:    "recording.path",
			Severity: "error",
			Message:  "recording path is required",
		})
	}

	if config.Destination.Mode != "external_rtmp" && config.Destination.Mode != "owncast" {
		issues = append(issues, ValidationIssue{
			Field:    "destination.mode",
			Severity: "error",
			Message:  "destination mode must be external_rtmp or owncast",
		})
	}

	knownRefs := make(map[string]struct{}, len(config.Sources)+len(config.Audio.Channels))
	seenSourceIDs := make(map[string]struct{}, len(config.Sources))
	for _, source := range config.Sources {
		if _, exists := seenSourceIDs[source.ID]; exists {
			issues = append(issues, ValidationIssue{
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
		if _, exists := seenAudioIDs[channel.ID]; exists {
			issues = append(issues, ValidationIssue{
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
		if _, exists := seenSceneIDs[scene.ID]; exists {
			issues = append(issues, ValidationIssue{
				Field:    "scenes",
				Severity: "error",
				Message:  "duplicate scene id: " + scene.ID,
			})
		}
		seenSceneIDs[scene.ID] = struct{}{}

		for _, ref := range scene.Sources {
			if _, exists := knownRefs[ref]; !exists {
				issues = append(issues, ValidationIssue{
					Field:    "scenes",
					Severity: "error",
					Message:  "scene " + scene.ID + " references unknown source or audio id: " + ref,
				})
			}
		}
	}

	return ValidateConfigResponse{
		Valid:  len(issues) == 0,
		Issues: issues,
	}
}
