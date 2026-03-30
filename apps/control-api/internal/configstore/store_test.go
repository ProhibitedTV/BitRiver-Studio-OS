package configstore

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

func TestStoreLoadsLegacyConfigAndPersistsSchema(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	now := time.Unix(0, 0).UTC()
	store, err := New(Paths{
		ActivePath: filepath.Join(tempDir, "active.json"),
		DraftPath:  filepath.Join(tempDir, "draft.json"),
	}, now)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	active := store.Active()
	if active.SchemaVersion != model.CurrentSchemaVersion {
		t.Fatalf("unexpected schema version: %s", active.SchemaVersion)
	}
	if active.Config.System.Name != "studio-a" {
		t.Fatalf("unexpected active config name: %s", active.Config.System.Name)
	}
}

func TestStoreApplyPromotesDraft(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	now := time.Unix(0, 0).UTC()
	store, err := New(Paths{
		ActivePath: filepath.Join(tempDir, "active.json"),
		DraftPath:  filepath.Join(tempDir, "draft.json"),
	}, now)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	config := model.DefaultConfig()
	config.System.Name = "studio-settings-test"

	if _, err := store.UpdateDraft(config); err != nil {
		t.Fatalf("update draft: %v", err)
	}

	response, validation, err := store.ApplyDraft(nil)
	if err != nil {
		t.Fatalf("apply draft: %v", err)
	}
	if validation != nil {
		t.Fatalf("unexpected validation failure: %+v", validation)
	}
	if response.AppliedGeneration == "" {
		t.Fatal("expected applied generation")
	}
	if store.Active().Config.System.Name != "studio-settings-test" {
		t.Fatalf("active config not updated: %s", store.Active().Config.System.Name)
	}
}
