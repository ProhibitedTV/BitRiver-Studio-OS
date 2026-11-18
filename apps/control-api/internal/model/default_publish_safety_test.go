package model

import "testing"

// Installing Studio OS must not publish to an external platform just because
// an example URL exists. All external destinations are opt-in.
func TestDefaultDestinationsAreDisabled(t *testing.T) {
	for name, config := range map[string]ControlConfig{
		"defaults": DefaultConfig(),
		"normalized empty": NormalizeConfig(ControlConfig{}),
		"normalized legacy destination": NormalizeConfig(ControlConfig{Destination: ConfigDestination{
			Mode: "external_rtmp", TargetLabel: "My instance", TargetURL: "rtmp://live.example.test/app",
		}}),
	} {
		if len(config.MediaRouter.PublishDestinations) == 0 {
			t.Fatalf("%s: expected a disabled example destination", name)
		}
		for _, destination := range config.MediaRouter.PublishDestinations {
			if destination.Enabled {
				t.Fatalf("%s: destination %q was enabled without explicit consent", name, destination.ID)
			}
		}
	}
}

func TestExplicitDestinationEnableIsNotReset(t *testing.T) {
	config := DefaultConfig()
	config.MediaRouter.PublishDestinations[0].Enabled = true
	normalized := NormalizeConfig(config)
	if !normalized.MediaRouter.PublishDestinations[0].Enabled {
		t.Fatal("explicitly enabled destination was unexpectedly disabled")
	}
}
