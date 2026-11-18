package model

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

const StreamKeyDirectory = "/var/lib/bitriver/secrets/stream-keys/"

var streamKeyDestinationID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ValidateStreamKeyFile accepts a *reference* to a local operator-controlled
// credential, never the secret itself. It is intentionally limited to one
// managed directory so a config author cannot trick the media worker into
// reading unrelated host files.
func ValidateStreamKeyFile(destination ConfigMediaDestination) error {
	if destination.StreamKeyFile == "" {
		return nil
	}
	if destination.Protocol != "rtmp" {
		return errors.New("stream-key files are supported for RTMP destinations only")
	}
	if !streamKeyDestinationID.MatchString(destination.ID) ||
		destination.StreamKeyFile != StreamKeyDirectory+destination.ID {
		return errors.New("stream-key file must match the destination ID in the managed secret directory")
	}
	parsed, err := url.Parse(destination.TargetURL)
	if err != nil || parsed == nil || (parsed.Scheme != "rtmp" && parsed.Scheme != "rtmps") ||
		parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		strings.ContainsAny(destination.TargetURL, "|[] \r\n\t") ||
		parsed.Path == "" || parsed.Path == "/" {
		return errors.New("stream-key destinations require an RTMP(S) server URL without credentials or query parameters")
	}
	return nil
}
