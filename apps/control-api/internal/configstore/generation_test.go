package configstore

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

func TestDraftGenerationPreventsConcurrentOperatorOverwrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store, err := New(Paths{
		ActivePath: filepath.Join(root, "active.json"),
		DraftPath: filepath.Join(root, "draft.json"),
	}, time.Unix(0, 0).UTC())
	if err != nil { t.Fatal(err) }
	original := store.Draft()
	firstConfig := original.Config
	firstConfig.System.Name = "creator-a"
	first, err := store.UpdateDraftIfGeneration(firstConfig, original.Generation)
	if err != nil { t.Fatal(err) }
	if first.Generation == original.Generation { t.Fatal("draft generation did not change") }

	staleConfig := original.Config
	staleConfig.System.Name = "creator-b"
	_, err = store.UpdateDraftIfGeneration(staleConfig, original.Generation)
	var apiErr *model.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 409 {
		t.Fatalf("stale editor must receive 409, got %v", err)
	}
	if got := store.Draft(); got.Config.System.Name != "creator-a" || got.Generation != first.Generation {
		t.Fatal("stale editor overwrote draft")
	}

	secondConfig := first.Config
	secondConfig.System.Name = "creator-a-2"
	second, err := store.UpdateDraftIfGeneration(secondConfig, first.Generation)
	if err != nil { t.Fatal(err) }
	if second.Generation == first.Generation { t.Fatal("rapid successive updates reused a generation") }

	appliedSideEffects := false
	_, validation, err := store.ApplyDraftIfGeneration(first.Generation, func(model.ConfigDocument) error {
		appliedSideEffects = true
		return nil
	})
	if !errors.As(err, &apiErr) || apiErr.Status != 409 || validation != nil {
		t.Fatalf("stale apply must receive 409, got %v", err)
	}
	if appliedSideEffects { t.Fatal("stale apply called media adapter") }
	if store.Active().Config.System.Name == "creator-a-2" { t.Fatal("stale apply promoted draft") }

	result, validation, err := store.ApplyDraftIfGeneration(second.Generation, nil)
	if err != nil || validation != nil { t.Fatalf("expected successful apply: %v, validation=%+v", err, validation) }
	if result.AppliedGeneration != store.Active().Generation { t.Fatal("mismatched applied generation") }
	if store.Active().Config.System.Name != "creator-a-2" { t.Fatal("correct draft was not applied") }
	if store.Draft().Generation == second.Generation || store.Draft().Generation == original.Generation {
		t.Fatal("post-apply draft should get a fresh generation")
	}
}

func TestDraftPreconditionDoesNotBlockLegacyCallers(t *testing.T) {
	t.Parallel()
	store, err := New(Paths{}, time.Unix(0, 0).UTC())
	if err != nil { t.Fatal(err) }
	config := model.DefaultConfig()
	config.System.Name = "legacy-editor"
	if _, err := store.UpdateDraft(config); err != nil { t.Fatal(err) }
	if _, validation, err := store.ApplyDraft(nil); err != nil || validation != nil {
		t.Fatalf("legacy apply failed: %v validation=%+v", err, validation)
	}
}
