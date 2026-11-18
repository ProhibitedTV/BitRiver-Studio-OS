package configstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

type Paths struct {
	ActivePath string
	DraftPath  string
}

type Store struct {
	mu     sync.RWMutex
	paths  Paths
	active model.ConfigDocument
	draft  model.ConfigDocument
}

func New(paths Paths, now time.Time) (*Store, error) {
	active, err := loadDocument(paths.ActivePath, "gen-bootstrap", now)
	if err != nil {
		return nil, fmt.Errorf("load active settings: %w", err)
	}

	draft, err := loadDocument(paths.DraftPath, "draft-next", now)
	if err != nil {
		return nil, fmt.Errorf("load draft settings: %w", err)
	}

	if draft.Generation == "" {
		draft = model.NewDocument("draft-next", now, active.ControlConfig)
	}

	store := &Store{
		paths:  paths,
		active: active,
		draft:  draft,
	}

	if err := store.persistUnlocked(paths.ActivePath, active); err != nil {
		return nil, err
	}
	if err := store.persistUnlocked(paths.DraftPath, draft); err != nil {
		return nil, err
	}

	return store, nil
}

func (store *Store) Active() model.ConfigEnvelope {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return model.EnvelopeFromDocument(store.active)
}

func (store *Store) Draft() model.ConfigEnvelope {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return model.EnvelopeFromDocument(store.draft)
}

func (store *Store) CurrentConfig() model.ControlConfig {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return model.CloneConfig(store.active.ControlConfig)
}

func (store *Store) UpdateDraft(config model.ControlConfig) (model.ConfigEnvelope, error) {
	return store.UpdateDraftIfGeneration(config, "")
}

// UpdateDraftIfGeneration prevents a stale client from silently overwriting
// a draft saved by another operator. The generation check and write share
// one mutex; clients supplying an expected generation receive HTTP 409 on
// conflicts. Empty expectedGeneration preserves existing API callers.
func (store *Store) UpdateDraftIfGeneration(config model.ControlConfig, expectedGeneration string) (model.ConfigEnvelope, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if expectedGeneration != "" && store.draft.Generation != expectedGeneration {
		return model.ConfigEnvelope{}, model.NewConflictError("draft settings changed; reload before saving")
	}
	now := time.Now().UTC()
	next := model.NewDocument("draft-"+now.Format("20060102-150405.000000000"), now, config)
	if err := store.persistUnlocked(store.paths.DraftPath, next); err != nil {
		return model.ConfigEnvelope{}, err
	}
	store.draft = next

	return model.EnvelopeFromDocument(store.draft), nil
}

func (store *Store) ValidateDraft() model.ValidateConfigResponse {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return ValidateConfig(store.draft.ControlConfig)
}

func (store *Store) ApplyDraft(beforeCommit func(model.ConfigDocument) error) (model.OperationResponse, *model.ValidateConfigResponse, error) {
	return store.ApplyDraftIfGeneration("", beforeCommit)
}

// ApplyDraftIfGeneration verifies that the validated draft is still the one
// the operator reviewed, before any media router side effects occur.
func (store *Store) ApplyDraftIfGeneration(expectedGeneration string, beforeCommit func(model.ConfigDocument) error) (model.OperationResponse, *model.ValidateConfigResponse, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	if expectedGeneration != "" && store.draft.Generation != expectedGeneration {
		return model.OperationResponse{}, nil, model.NewConflictError("draft settings changed after validation; reload and review again")
	}

	validation := ValidateConfig(store.draft.ControlConfig)
	if !validation.Valid {
		return model.OperationResponse{}, &validation, nil
	}

	now := time.Now().UTC()
	nextActive := model.NewDocument("gen-"+now.Format("20060102-150405.000000000"), now, store.draft.ControlConfig)
	nextDraft := model.NewDocument("draft-"+now.Format("20060102-150405.000000000"), now, nextActive.ControlConfig)

	if beforeCommit != nil {
		if err := beforeCommit(nextActive); err != nil {
			return model.OperationResponse{}, nil, err
		}
	}

	store.active = nextActive
	store.draft = nextDraft

	if err := store.persistUnlocked(store.paths.ActivePath, store.active); err != nil {
		return model.OperationResponse{}, nil, err
	}
	if err := store.persistUnlocked(store.paths.DraftPath, store.draft); err != nil {
		return model.OperationResponse{}, nil, err
	}

	return model.OperationResponse{
		OperationID:       nextOperationID("settings_apply"),
		Status:            model.OperationCompleted,
		Message:           "draft promoted to active settings",
		AppliedGeneration: store.active.Generation,
	}, nil, nil
}

func (store *Store) persistUnlocked(path string, document model.ConfigDocument) error {
	if path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings document: %w", err)
	}

	if err := os.WriteFile(path, append(payload, '\n'), 0o640); err != nil {
		return fmt.Errorf("write settings document: %w", err)
	}

	return nil
}

func loadDocument(path string, fallbackGeneration string, now time.Time) (model.ConfigDocument, error) {
	if path == "" {
		return model.NewDocument(fallbackGeneration, now, model.DefaultConfig()), nil
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.NewDocument(fallbackGeneration, now, model.DefaultConfig()), nil
		}
		return model.ConfigDocument{}, err
	}

	var document model.ConfigDocument
	if err := json.Unmarshal(payload, &document); err == nil && document.Generation != "" {
		document.SchemaVersion = model.CurrentSchemaVersion
		if document.UpdatedAt.IsZero() {
			document.UpdatedAt = now.UTC()
		}
		document.ControlConfig = model.NormalizeConfig(document.ControlConfig)
		return document, nil
	}

	var legacy model.ControlConfig
	if err := json.Unmarshal(payload, &legacy); err == nil && legacy.System.Name != "" {
		return model.NewDocument(fallbackGeneration, now, legacy), nil
	}

	return model.ConfigDocument{}, fmt.Errorf("unrecognized settings format at %s", path)
}

var operationSequence uint64

func nextOperationID(prefix string) string {
	return fmt.Sprintf("%s-%06d", prefix, atomic.AddUint64(&operationSequence, 1))
}
