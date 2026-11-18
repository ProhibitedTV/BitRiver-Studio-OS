package model

import "testing"

func TestStreamKeyReferenceUsesManagedPathAndCredentialFreeURL(t *testing.T) {
	t.Parallel()
	good := ConfigMediaDestination{
		ID:            "bitriver-live",
		Protocol:      "rtmp",
		TargetURL:     "rtmp://ingest.example.test:1935/live",
		StreamKeyFile: StreamKeyDirectory + "bitriver-live",
	}
	if err := ValidateStreamKeyFile(good); err != nil {
		t.Fatal(err)
	}
	cases := []ConfigMediaDestination{
		{ID: "bitriver-live", Protocol: "srt", TargetURL: good.TargetURL, StreamKeyFile: good.StreamKeyFile},
		{ID: "bitriver-live", Protocol: "rtmp", TargetURL: good.TargetURL, StreamKeyFile: "/etc/shadow"},
		{ID: "..", Protocol: "rtmp", TargetURL: good.TargetURL, StreamKeyFile: StreamKeyDirectory + ".."},
		{ID: good.ID, Protocol: "rtmp", TargetURL: "rtmp://user:pass@ingest.example.test/live", StreamKeyFile: good.StreamKeyFile},
		{ID: good.ID, Protocol: "rtmp", TargetURL: "rtmp://ingest.example.test/live?key=hidden", StreamKeyFile: good.StreamKeyFile},
		{ID: good.ID, Protocol: "rtmp", TargetURL: "rtmp://ingest.example.test/live|bad", StreamKeyFile: good.StreamKeyFile},
		{ID: good.ID, Protocol: "rtmp", TargetURL: "http://ingest.example.test/live", StreamKeyFile: good.StreamKeyFile},
		{ID: good.ID, Protocol: "rtmp", TargetURL: "rtmp://ingest.example.test", StreamKeyFile: good.StreamKeyFile},
	}
	for _, tc := range cases {
		if err := ValidateStreamKeyFile(tc); err == nil {
			t.Errorf("unexpectedly accepted credential reference for %q", tc.ID)
		}
	}
	legacy := ConfigMediaDestination{ID: "existing", Protocol: "rtmp", TargetURL: "rtmp://example.com/app"}
	if err := ValidateStreamKeyFile(legacy); err != nil {
		t.Fatalf("legacy destination should remain usable: %v", err)
	}
}
