//go:build linux

package operatorcredentials

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"syscall"
)

func writeFixture(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "admin-credentials.json")
	key := "existing-operator-key"
	sum := sha256.Sum256([]byte(key))
	original := map[string]any{
		"username": "admin", "passwordHash": "$6$legacy", "controlKeySha256": hex.EncodeToString(sum[:]),
		"rotateOnFirstLogin": true,
	}
	data, err := json.Marshal(original)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, data, 0o600); err != nil { t.Fatal(err) }
	return path, key
}

func TestRotateRetainsCredentialsAndFileSecurity(t *testing.T) {
	t.Parallel()
	path, oldKey := writeFixture(t)
	originalInfo, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	expectedEntropy := strings.Repeat("\x5a", 32)
	key, err := rotate(path, strings.NewReader(expectedEntropy))
	if err != nil { t.Fatal(err) }
	if key != hex.EncodeToString([]byte(expectedEntropy)) {
		t.Fatal("unexpected rotated key")
	}
	digest, err := ReadDigest(path)
	if err != nil { t.Fatal(err) }
	expected := sha256.Sum256([]byte(key))
	if digest != expected { t.Fatal("stored digest does not match new key") }
	if digest == sha256.Sum256([]byte(oldKey)) { t.Fatal("old key remains valid") }

	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("rotated credential file permissions: %#o", info.Mode().Perm())
	}
	oldOwner, oldOK := originalInfo.Sys().(*syscall.Stat_t)
	newOwner, newOK := info.Sys().(*syscall.Stat_t)
	if !oldOK || !newOK || oldOwner.Uid != newOwner.Uid || oldOwner.Gid != newOwner.Gid {
		t.Fatal("rotation changed the credentials owner or group")
	}

	content, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	var fields map[string]any
	if err := json.Unmarshal(content, &fields); err != nil { t.Fatal(err) }
	if fields["username"] != "admin" || fields["passwordHash"] != "$6$legacy" || fields["rotateOnFirstLogin"] != true {
		t.Fatalf("rotation damaged existing fields: %v", fields)
	}
	if fields["operatorKeyRotatedAt"] == nil {
		t.Fatal("missing audit timestamp")
	}
}

func TestRotationFailsClosedWithoutChangingOriginal(t *testing.T) {
	t.Parallel()
	path, _ := writeFixture(t)
	original, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }

	for _, input := range []string{"", "bad", strings.Repeat("a", 31)} {
		_, err := rotate(path, strings.NewReader(input))
		if err == nil { t.Fatalf("accepted inadequate random input %q", input) }
		after, _ := os.ReadFile(path)
		if string(after) != string(original) { t.Fatal("failed rotation modified credentials") }
	}

	if err := os.Chmod(path, 0o644); err != nil { t.Fatal(err) }
	if _, err := Rotate(path); err == nil { t.Fatal("accepted world-readable credential file") }
	if err := os.Chmod(path, 0o600); err != nil { t.Fatal(err) }

	symlink := filepath.Join(filepath.Dir(path), "linked.json")
	if err := os.Symlink(path, symlink); err != nil { t.Fatal(err) }
	if _, err := Rotate(symlink); err == nil { t.Fatal("followed credential symlink") }

	if err := os.WriteFile(path, []byte("{corrupt"), 0o600); err != nil { t.Fatal(err) }
	if _, err := Rotate(path); err == nil { t.Fatal("accepted corrupt credentials") }
}
