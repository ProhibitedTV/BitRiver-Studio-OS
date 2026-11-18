package model

import "testing"

func TestValidatePublishTarget(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		protocol string
		url      string
		valid    bool
	}{
		{"bitriver live", "rtmp", "rtmp://ingest.example.test:1935/live", true},
		{"custom rtmps", "rtmp", "rtmps://live.example.test/app/abc", true},
		{"srt stream id", "srt", "srt://relay.example.test:9000?streamid=publish:backup", true},
		{"tee output injection", "rtmp", "rtmp://ingest.example.test/live|rtmp://attacker.test/live", false},
		{"tee option injection", "srt", "srt://relay.example.test:9000/[f=flv]rtmp://evil.test/live", false},
		{"backslash escape", "rtmp", "rtmp://ingest.example.test/live\\|evil", false},
		{"space", "rtmp", "rtmp://ingest.example.test/live?foo=bar baz", false},
		{"user info", "rtmp", "rtmp://user:pass@ingest.example.test/live", false},
		{"fragment", "rtmp", "rtmp://ingest.example.test/live#other", false},
		{"no host", "rtmp", "rtmp:///live", false},
		{"no application", "rtmp", "rtmp://ingest.example.test", false},
		{"wrong scheme", "srt", "https://relay.example.test/live", false},
		{"unrecognized protocol", "xyz", "rtmp://ingest.example.test/live", false},
		{"too long", "rtmp", "rtmp://ingest.example.test/live/" + string(make([]byte, 2100)), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePublishTarget(ConfigMediaDestination{
				ID: "test", Protocol: tc.protocol, TargetURL: tc.url,
			})
			if (err == nil) != tc.valid {
				t.Fatalf("expected valid=%v for %q, got error %v", tc.valid, tc.url, err)
			}
		})
	}
}
