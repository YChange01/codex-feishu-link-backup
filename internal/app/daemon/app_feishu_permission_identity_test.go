package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/YChange01/codex-feishu-link/internal/adapter/feishu"
	"github.com/YChange01/codex-feishu-link/internal/config"
)

func TestPermissionGrantVerificationSkipsBusyLifecycleAndRecoversOnRetry(t *testing.T) {
	gateway := &permissionClearingGateway{}
	app := New(":0", ":0", gateway, serverIdentityForTest())
	cfg := feishu.LiveGatewayConfig{GatewayID: "main", AppID: "app", AppSecret: "secret"}
	app.admin.loadConfig = func() (config.LoadedAppConfig, error) {
		return config.LoadedAppConfig{Config: config.AppConfig{Feishu: config.FeishuSettings{Apps: []config.FeishuAppConfig{{ID: cfg.GatewayID, AppID: cfg.AppID, AppSecret: cfg.AppSecret}}}}}, nil
	}
	app.observeFeishuPermissionError("main", &feishu.APIError{API: "drive.v1.file.list", Code: 99991672, Msg: "missing drive:drive", PermissionViolations: []feishu.APIErrorPermissionViolation{{Type: "tenant", Subject: "drive:drive"}}})
	previous := listFeishuAppGrantedScopes
	t.Cleanup(func() { listFeishuAppGrantedScopes = previous })
	listFeishuAppGrantedScopes = func(context.Context, feishu.LiveGatewayConfig) ([]feishu.AppScopeStatus, error) {
		return []feishu.AppScopeStatus{{ScopeName: "drive:drive", ScopeType: "tenant", GrantStatus: 1}}, nil
	}
	// Replacement may hold this lock while waiting for the action whose
	// permission refresh is executing below. Refresh must release that action.
	app.feishuApplyMu.Lock()
	done := make(chan struct{})
	go func() { app.verifyFeishuPermissionGaps(context.Background(), cfg); close(done) }()
	returned := false
	select {
	case <-done:
		returned = true
	case <-time.After(time.Second):
	}
	app.feishuApplyMu.Unlock()
	if !returned {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("verification remained stuck after lifecycle unlock")
		}
		t.Fatal("verification waited for lifecycle lock and could deadlock action drain")
	}
	if len(app.snapshotFeishuPermissionGaps("main")) != 1 || len(gateway.clearCalls) != 0 {
		t.Fatal("busy lifecycle verification cleared gap or broker")
	}
	app.verifyFeishuPermissionGaps(context.Background(), cfg)
	if len(app.snapshotFeishuPermissionGaps("main")) != 0 || len(gateway.clearCalls) != 1 {
		t.Fatal("verification did not recover after lifecycle became idle")
	}
}

func TestPermissionGrantQueryCannotClearReplacementAppOrCredentials(t *testing.T) {
	for _, change := range []string{"app identity", "credentials", "configured app pending apply", "active domain changed"} {
		t.Run(change, func(t *testing.T) {
			gateway := &permissionClearingGateway{}
			app := New(":0", ":0", gateway, serverIdentityForTest())
			oldConfig := feishu.LiveGatewayConfig{GatewayID: "main", AppID: "old-app", AppSecret: "old-secret"}
			currentApp := config.FeishuAppConfig{ID: "main", AppID: oldConfig.AppID, AppSecret: oldConfig.AppSecret}
			app.admin.loadConfig = func() (config.LoadedAppConfig, error) {
				return config.LoadedAppConfig{Config: config.AppConfig{Feishu: config.FeishuSettings{Apps: []config.FeishuAppConfig{currentApp}}}}, nil
			}
			errorEvidence := &feishu.APIError{API: "drive.v1.file.list", Code: 99991672, Msg: "missing drive:drive", PermissionViolations: []feishu.APIErrorPermissionViolation{{Type: "tenant", Subject: "drive:drive"}}}
			app.observeFeishuPermissionError("main", errorEvidence)
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			previous := listFeishuAppGrantedScopes
			t.Cleanup(func() { listFeishuAppGrantedScopes = previous })
			listFeishuAppGrantedScopes = func(context.Context, feishu.LiveGatewayConfig) ([]feishu.AppScopeStatus, error) {
				close(entered)
				<-release
				return []feishu.AppScopeStatus{{ScopeName: "drive:drive", ScopeType: "tenant", GrantStatus: 1}}, nil
			}
			go func() { app.verifyFeishuPermissionGaps(context.Background(), oldConfig); close(done) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("grant query did not start")
			}
			// This is the same serialization boundary used by runtime replacement.
			app.feishuApplyMu.Lock()
			switch change {
			case "app identity":
				currentApp.AppID = "replacement-app"
			case "credentials":
				currentApp.AppSecret = "replacement-secret"
			case "configured app pending apply":
				// Persisted cfg matches the query, but the active worker is still
				// the previous app while applying the new config is pending.
				gateway.permissionOwner = &feishu.LiveGatewayConfig{AppID: "active-previous-app", AppSecret: oldConfig.AppSecret}
			case "active domain changed":
				gateway.permissionOwner = &feishu.LiveGatewayConfig{AppID: oldConfig.AppID, AppSecret: oldConfig.AppSecret, Domain: "https://open.larksuite.com"}
			}
			app.clearFeishuPermissionGaps("main")
			app.observeFeishuPermissionError("main", errorEvidence)
			app.feishuApplyMu.Unlock()
			close(release)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("grant verification did not finish")
			}
			if len(app.snapshotFeishuPermissionGaps("main")) != 1 {
				t.Fatal("old app evidence cleared replacement gap")
			}
			if len(gateway.clearCalls) != 0 {
				t.Fatal("old app evidence reached replacement broker")
			}
		})
	}
}
