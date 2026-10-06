package feishu

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	previewpkg "github.com/YChange01/codex-feishu-link/internal/adapter/feishu/preview"
)

func TestPreviewPermissionFailureBlocksUntilMatchingGrantAndRecovers(t *testing.T) {
	for _, useGrant := range []bool{true, false} {
		t.Run(map[bool]string{true: "matching grant", false: "ttl probe"}[useGrant], func(t *testing.T) {
			testPreviewPermissionRecovery(t, useGrant)
		})
	}
}

func testPreviewPermissionRecovery(t *testing.T, useGrant bool) {
	fileCalls := 0
	granted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"fake-token"}`))
		case "/open-apis/drive/v1/files":
			fileCalls++
			if !granted {
				_, _ = w.Write([]byte(`{"code":99991672,"msg":"Access denied. One of the following scopes is required: [drive:drive]"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"files":[],"has_more":false}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "main", AppID: server.URL, AppSecret: "fake-secret", Domain: server.URL})
	current := time.Now()
	gateway.broker.now = func() time.Time { return current }
	gateway.broker.sleep = func(_ context.Context, delay time.Duration) error { current = current.Add(delay); return nil }
	controller := NewMultiGatewayController()
	controller.workers["main"] = &gatewayWorker{runtime: gateway, generation: 1, actionGate: newGatewayActionGate()}
	observed, successful := 0, 0
	controller.SetPermissionResultHook(func(gatewayID, api string, err error) {
		if err == nil {
			successful++
			return
		}
		observed++
		// Calling back into the controller must not deadlock.
		controller.Status()
		gap, ok := ExtractPermissionGap(err)
		if gatewayID != "main" || api != "drive.v1.file.list" || !ok || gap.Scope != "drive:drive" || gap.ScopeType != "tenant" {
			t.Errorf("unexpected permission evidence: gateway=%s gap=%#v", gatewayID, gap)
		}
	})
	preview := controller.newPreviewer(gateway, GatewayAppConfig{GatewayID: "main", PreviewCacheDir: t.TempDir(), PreviewStatePath: filepath.Join(t.TempDir(), "preview.json")}).(previewpkg.PreviewDriveAdminService)
	if _, err := preview.CleanupBefore(context.Background(), time.Time{}); err == nil {
		t.Fatal("expected actual API permission failure")
	}
	if observed != 1 || fileCalls != 1 {
		t.Fatalf("first failure: observed=%d fileCalls=%d", observed, fileCalls)
	}
	controller.ClearGrantedPermissionBlocks("main", []AppScopeStatus{{ScopeName: "drive:drive", ScopeType: "user", GrantStatus: 1}})
	_, err := preview.CleanupBefore(context.Background(), time.Time{})
	var blocked *PermissionBlockedError
	if !errors.As(err, &blocked) || fileCalls != 1 {
		t.Fatalf("wrong token grant must keep shared block: err=%v calls=%d", err, fileCalls)
	}
	if !previewpkg.IsDriveAccessDeniedError(err) {
		t.Fatalf("blocked calls must preserve actionable Drive permission classification: %v", err)
	}
	granted = true
	if useGrant {
		controller.ClearGrantedPermissionBlocks("main", []AppScopeStatus{{ScopeName: "drive:drive", ScopeType: "tenant", GrantStatus: 1}})
	} else {
		current = current.Add(callBrokerPermissionBlockTTL)
	}
	if _, err := preview.CleanupBefore(context.Background(), time.Time{}); err != nil {
		t.Fatalf("matching grant must restore preview API: %v", err)
	}
	if successful != 2 {
		t.Fatalf("successful API calls not reported: %d", successful)
	}
	if fileCalls != 3 {
		t.Fatalf("expected cleanup and summary HTTP requests after recovery, got %d", fileCalls)
	}
	// A retired worker cannot report permission gaps into its replacement.
	controller.workers["main"].generation++
	granted = false
	_, _ = preview.CleanupBefore(context.Background(), time.Time{})
	if observed != 1 {
		t.Fatalf("retired worker leaked permission evidence, observed=%d", observed)
	}
}
