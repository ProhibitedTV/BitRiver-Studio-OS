// Package operatorcredentials reads and rotates pre-release operator control keys.
// Keys are random 256-bit bearer secrets, not user-chosen passwords.
package operatorcredentials

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func read(path string) (map[string]json.RawMessage, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, fmt.Errorf("invalid operator credentials: %w", err)
	}
	if fields == nil {
		return nil, errors.New("operator credentials must be an object")
	}
	return fields, nil
}

// ReadDigest validates the active key digest without exposing the bearer key.
func ReadDigest(path string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	fields, err := read(path)
	if err != nil {
		return digest, err
	}
	var encoded string
	if err := json.Unmarshal(fields["controlKeySha256"], &encoded); err != nil {
		return digest, errors.New("operator credentials missing controlKeySha256")
	}
	value, err := hex.DecodeString(encoded)
	if err != nil || len(value) != sha256.Size {
		return digest, errors.New("operator credentials have invalid controlKeySha256")
	}
	copy(digest[:], value)
	return digest, nil
}

// Rotate replaces only the control-key digest, retaining the rest of the
// firstboot record. The caller MUST show/store the returned key securely.
// Run as root or as the owner of the credentials file.
func Rotate(path string) (string, error) {
	return rotate(path, rand.Reader)
}

func rotate(path string, entropy io.Reader) (string, error) {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !fileInfo.Mode().IsRegular() {
		return "", errors.New("operator credential file must be regular (no symlinks)")
	}
	if fileInfo.Mode().Perm()&0o077 != 0 {
		return "", errors.New("operator credential file must not be readable by group or others")
	}
	fields, err := read(path)
	if err != nil {
		return "", err
	}
	if _, err := ReadDigest(path); err != nil {
		return "", err
	}

	raw := make([]byte, 32)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("randomness unavailable: %w", err)
	}
	key := hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(key))
	fields["controlKeySha256"], _ = json.Marshal(hex.EncodeToString(digest[:]))
	fields["operatorKeyRotatedAt"], _ = json.Marshal(time.Now().UTC().Format(time.RFC3339Nano))
	payload, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')

	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, ".operator-credentials-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := temp.Chmod(0o600); err != nil {
		return "", err
	}
	// A root-run recovery command must not transfer ownership of the file
	// away from the bitriver service user.
	if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		uid, gid := int(stat.Uid), int(stat.Gid)
		if uid != os.Geteuid() || gid != os.Getegid() {
			if err := temp.Chown(uid, gid); err != nil {
				return "", fmt.Errorf("cannot preserve credentials ownership: %w", err)
			}
		}
	}
	if _, err := temp.Write(payload); err != nil {
		return "", err
	}
	if err := temp.Sync(); err != nil {
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return "", err
	}
	return key, nil
}
