package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YChange01/codex-feishu-link/internal/adapter/feishu"
	"github.com/YChange01/codex-feishu-link/internal/app/daemon/feishufacts"
	"github.com/YChange01/codex-feishu-link/internal/config"
	"github.com/YChange01/codex-feishu-link/internal/core/orchestrator"
)

func TestConfiguredReadinessAndObservedPermissionGapsUseSeparateEvidence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		knownGap bool
		grants   []feishu.AppScopeStatus
		grantErr error
		wantGap  bool
	}{
		{name: "no observed gap", knownGap: false},
		{name: "omitted grants preserve denial", knownGap: true, wantGap: true},
		{name: "query failure preserves denial and readiness", knownGap: true, grantErr: errors.New("grant query unavailable"), wantGap: true},
		{name: "user grant cannot clear tenant denial", knownGap: true, grants: []feishu.AppScopeStatus{{ScopeName: "drive:drive", ScopeType: "user", GrantStatus: 1}}, wantGap: true},
		{name: "positive tenant grant clears denial", knownGap: true, grants: []feishu.AppScopeStatus{{ScopeName: "drive:drive", ScopeType: "tenant", GrantStatus: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gateway := &permissionClearingGateway{}
			app := New(":0", ":0", gateway, serverIdentityForTest())
			app.configureFeishuFactsStateLocked(t.TempDir())
			app.admin.loadConfig = func() (config.LoadedAppConfig, error) {
				return config.LoadedAppConfig{Config: config.AppConfig{Feishu: config.FeishuSettings{Apps: []config.FeishuAppConfig{{ID: "main", AppID: "fake-app", AppSecret: "fake-secret"}}}}}, nil
			}
			if tc.knownGap {
				gateway.permissionResultHook("main", "drive.v1.file.list", &feishu.APIError{API: "drive.v1.file.list", Code: 99991672, Msg: "missing drive:drive", PermissionViolations: []feishu.APIErrorPermissionViolation{{Type: "tenant", Subject: "drive:drive"}}})
			}
			previousConfigured, previousGranted, previousBot := listFeishuAppConfiguredScopes, listFeishuAppGrantedScopes, getFeishuBotInfo
			t.Cleanup(func() {
				listFeishuAppConfiguredScopes, listFeishuAppGrantedScopes, getFeishuBotInfo = previousConfigured, previousGranted, previousBot
			})
			listFeishuAppConfiguredScopes = func(context.Context, feishu.LiveGatewayConfig) ([]feishu.AppScopeStatus, error) {
				return []feishu.AppScopeStatus{{ScopeName: "drive:drive", ScopeType: "tenant", GrantStatus: 1}, {ScopeName: "im:message.group_msg", ScopeType: "tenant", GrantStatus: 1}}, nil
			}
			grantCalls := 0
			listFeishuAppGrantedScopes = func(context.Context, feishu.LiveGatewayConfig) ([]feishu.AppScopeStatus, error) {
				grantCalls++
				return tc.grants, tc.grantErr
			}
			getFeishuBotInfo = func(context.Context, feishu.LiveGatewayConfig) (feishu.BotInfo, error) { return feishu.BotInfo{}, nil }
			record, err := app.RefreshFeishuBotFacts(context.Background(), "main")
			if err != nil || record.ScopesError != "" {
				t.Fatalf("grant verification polluted configured readiness: err=%v scopesError=%s", err, record.ScopesError)
			}
			wantCalls := 0
			if tc.knownGap {
				wantCalls = 1
			}
			if grantCalls != wantCalls {
				t.Fatalf("grant queries=%d, want %d", grantCalls, wantCalls)
			}
			if hasGap := len(app.snapshotFeishuPermissionGaps("main")) > 0; hasGap != tc.wantGap {
				t.Fatalf("gap=%t, want %t", hasGap, tc.wantGap)
			}
			decision := app.CheckPrimaryBotPermission(context.Background(), orchestrator.PrimaryBotPermissionRequest{GatewayID: "main"})
			if !decision.Allowed {
				t.Fatalf("omitted tenant grants denied configured self-built app: %#v", decision)
			}
			if tc.grantErr != nil && len(gateway.clearCalls) != 0 {
				t.Fatal("failed grant query cleared broker blocks")
			}
		})
	}
}

func TestPermissionCheckRefreshesPreStartupScopesEvenWithinTTL(t *testing.T) {
	for _, primary := range []bool{false, true} {
		t.Run(map[bool]string{false: "feature", true: "primary"}[primary], func(t *testing.T) {
			app := New(":0", ":0", &recordingGateway{}, serverIdentityForTest())
			app.daemonStartedAt = time.Now().UTC()
			app.configureFeishuFactsStateLocked(t.TempDir())
			app.admin.loadConfig = func() (config.LoadedAppConfig, error) {
				return config.LoadedAppConfig{Config: config.AppConfig{Feishu: config.FeishuSettings{Apps: []config.FeishuAppConfig{{ID: "main", AppID: "fake-app", AppSecret: "fake-secret"}}}}}, nil
			}
			if err := app.feishuFactsState.store.Put(feishufacts.Record{
				GatewayID: "main", AppID: "fake-app", ScopesFetchedAt: app.daemonStartedAt.Add(-time.Second),
				Scopes: []feishufacts.ScopeStatus{{ScopeName: "im:message.group_msg", ScopeType: "tenant", GrantStatus: 1}},
			}); err != nil {
				t.Fatal(err)
			}
			previousScopes, previousBot := listFeishuAppConfiguredScopes, getFeishuBotInfo
			t.Cleanup(func() { listFeishuAppConfiguredScopes, getFeishuBotInfo = previousScopes, previousBot })
			calls := 0
			listFeishuAppConfiguredScopes = func(context.Context, feishu.LiveGatewayConfig) ([]feishu.AppScopeStatus, error) {
				calls++
				return nil, nil
			}
			getFeishuBotInfo = func(context.Context, feishu.LiveGatewayConfig) (feishu.BotInfo, error) { return feishu.BotInfo{}, nil }
			var decision orchestrator.PrimaryBotPermissionDecision
			if primary {
				decision = app.CheckPrimaryBotPermission(context.Background(), orchestrator.PrimaryBotPermissionRequest{GatewayID: "main"})
			} else {
				decision = app.checkFeishuScopePermission(context.Background(), "main", "primary_room_bot", false)
			}
			if calls != 1 || decision.Allowed {
				t.Fatalf("pre-startup grants trusted: calls=%d decision=%#v", calls, decision)
			}
		})
	}
}
