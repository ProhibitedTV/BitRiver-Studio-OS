package model

import (
	"errors"
	"net/url"
	"strings"
)

// ValidatePublishTarget prevents an RTMP/SRT destination from injecting extra
// outputs or tee options into FFmpeg's multiplexed output URL. This applies
// even without a streamKeyFile (legacy/custom destinations).
func ValidatePublishTarget(destination ConfigMediaDestination) error {
	raw := destination.TargetURL
	if raw == "" || len(raw) > 2048 || strings.ContainsAny(raw, "|[]\\ \r\n\t") {
		return errors.New("publish URL contains invalid characters or exceeds 2048 bytes")
	}
	target, err := url.Parse(raw)
	if err != nil || target == nil || target.Host == "" || target.User != nil ||
		target.Fragment != "" || target.Opaque != "" {
		return errors.New("publish URL must be a well-formed URL with a host and no embedded userinfo or fragment")
	}
	switch destination.Protocol {
	case "rtmp":
		if target.Scheme != "rtmp" && target.Scheme != "rtmps" {
			return errors.New("RTMP destination requires an rtmp:// or rtmps:// URL")
		}
		if target.Path == "" || target.Path == "/" {
			return errors.New("RTMP destination requires a server application path")
		}
	case "srt":
		if target.Scheme != "srt" {
			return errors.New("SRT destination requires an srt:// URL")
		}
	default:
		return errors.New("unsupported publish transport")
	}
	return nil
}
