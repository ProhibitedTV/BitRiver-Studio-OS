package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bitriver/studio-os/apps/control-api/internal/adapters"
	"github.com/bitriver/studio-os/apps/control-api/internal/model"
)

// testMediaRouter keeps the existing mock's ApplyConfig behavior, but lets
// tests simulate direct network contributors not visible in OBS.
type testMediaRouter struct {
	adapters.MediaMTX
	snapshot adapters.MediaMTXSnapshot
	snapshotErr error
	applies int
}

func (router *testMediaRouter) Snapshot(context.Context) (adapters.MediaMTXSnapshot, error) {
	if router.snapshotErr != nil {
		return adapters.MediaMTXSnapshot{}, router.snapshotErr
	}
	return router.snapshot, nil
}

func (router *testMediaRouter) ApplyConfig(ctx context.Context, generation string, config model.ControlConfig) (adapters.MediaApplyResult, error) {
	router.applies++
	return router.MediaMTX.ApplyConfig(ctx, generation, config)
}

func TestMediaRouterActiveSessionDetection(t *testing.T) {
	cases := []struct {
		name string
		snapshot adapters.MediaMTXSnapshot
		busy bool
	}{
		{"no sessions", adapters.MediaMTXSnapshot{}, false},
		{"unready path with reserved reader", adapters.MediaMTXSnapshot{Paths: []adapters.MediaPath{{Ready: false, Readers: 1}}}, false},
		{"live publisher", adapters.MediaMTXSnapshot{Paths: []adapters.MediaPath{{Ready: true, Publisher: true}}}, true},
		{"live viewer", adapters.MediaMTXSnapshot{Paths: []adapters.MediaPath{{Ready: true, Readers: 1}}}, true},
		{"network session", adapters.MediaMTXSnapshot{Sessions: []adapters.MediaSession{{ID: "guest-1", Protocol: "srt"}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if result := mediaRouterHasActiveSessions(tc.snapshot); result != tc.busy {
				t.Fatalf("expected busy %v, got %v", tc.busy, result)
			}
		})
	}
}

func TestSettingsApplyPreservesDraftDuringActiveMediaMTXSessions(t *testing.T) {
	t.Parallel()
	for _, activity := range []struct {
		name string
		update func(*adapters.MediaMTXSnapshot)
	}{
		{"direct SRT contributor", func(s *adapters.MediaMTXSnapshot) {
			s.Sessions = []adapters.MediaSession{{ID: "remote-camera", Path: "guest-1", Protocol: "srt", State: "publishing"}}
		}},
		{"live RTSP publisher", func(s *adapters.MediaMTXSnapshot) {
			s.Paths = append(s.Paths, adapters.MediaPath{Name: "guest-2", Ready: true, Publisher: true})
		}},
		{"active viewer", func(s *adapters.MediaMTXSnapshot) {
			s.Paths = append(s.Paths, adapters.MediaPath{Name: "program-preview", Ready: true, Readers: 2})
		}},
	} {
		t.Run(activity.name, func(t *testing.T) {
			ctx := context.Background()
			controller := newTestController(t)
			active := controller.ActiveConfig()
			draft := active.Config
			draft.System.Name = "updated-production-settings"
			if _, err := controller.UpdateDraft(draft); err != nil {
				t.Fatal(err)
			}
			snapshot, err := controller.mediaMTX.Snapshot(ctx)
			if err != nil { t.Fatal(err) }
			activity.update(&snapshot)
			router := &testMediaRouter{MediaMTX: controller.mediaMTX, snapshot: snapshot}
			controller.mediaMTX = router
			_, validation, err := controller.ApplyDraft(ctx)
			if validation != nil { t.Fatalf("unexpected validation result: %+v", validation) }
			var apiErr *model.APIError
			if !errors.As(err, &apiErr) || apiErr.Status != 409 {
				t.Fatalf("wanted HTTP 409 while %s; got %v", activity.name, err)
			}
			if router.applies != 0 { t.Fatal("active router was reconfigured") }
			if controller.ActiveConfig().Generation != active.Generation { t.Fatal("active config changed on rejected apply") }
			if controller.DraftConfig().Config.System.Name != draft.System.Name { t.Fatal("rejected apply discarded draft") }
			// After the contributor leaves, the same draft can be safely applied.
			router.snapshot.Sessions = nil
			for i := range router.snapshot.Paths {
				router.snapshot.Paths[i].Ready = false
				router.snapshot.Paths[i].Readers = 0
				router.snapshot.Paths[i].Publisher = false
			}
			_, validation, err = controller.ApplyDraft(ctx)
			if err != nil || validation != nil { t.Fatalf("off-air apply failed: %v; validation: %+v", err, validation) }
			if router.applies != 1 { t.Fatalf("wanted one off-air router apply, got %d", router.applies) }
		})
	}
}

func TestSettingsApplyFailsClosedWhenMediaMTXStatusUnavailable(t *testing.T) {
	t.Parallel()
	controller := newTestController(t)
	original := controller.ActiveConfig().Generation
	controller.mediaMTX = &testMediaRouter{
		MediaMTX: controller.mediaMTX,
		snapshotErr: errors.New("router API timeout"),
	}
	_, validation, err := controller.ApplyDraft(context.Background())
	if validation != nil { t.Fatalf("unexpected validation error: %+v", validation) }
	var apiErr *model.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 503 {
		t.Fatalf("expected HTTP 503 for unknown router activity, got %v", err)
	}
	if controller.ActiveConfig().Generation != original {
		t.Fatal("MediaMTX status failure altered active config")
	}
}
