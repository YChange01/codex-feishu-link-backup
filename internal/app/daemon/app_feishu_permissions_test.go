package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YChange01/codex-feishu-link/internal/adapter/feishu"
	"github.com/YChange01/codex-feishu-link/internal/core/agentproto"
)

func TestApplyFeishuPermissionVerificationResultClearsGrantedGap(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, serverIdentityForTest())
	if !app.observeFeishuPermissionError("app-1", &feishu.APIError{
		API:  "im.v1.message.create",
		Code: 99990001,
		Msg:  "permission denied",
		PermissionViolations: []feishu.APIErrorPermissionViolation{
			{Type: "tenant", Subject: "drive:drive"},
		},
	}) {
		t.Fatal("expected permission gap to be recorded")
	}

	app.applyFeishuPermissionVerificationResult("app-1", []feishu.AppScopeStatus{
		{ScopeName: "drive:drive", ScopeType: "tenant", GrantStatus: 1},
	}, nil)

	if got := app.snapshotFeishuPermissionGaps("app-1"); len(got) != 0 {
		t.Fatalf("expected granted scope to clear gap, got %#v", got)
	}
}

func TestApplyFeishuPermissionVerificationResultKeepsGapOnVerifyFailure(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, serverIdentityForTest())
	if !app.observeFeishuPermissionError("app-1", &feishu.APIError{
		API:  "im.v1.message.create",
		Code: 99990001,
		Msg:  "permission denied",
		PermissionViolations: []feishu.APIErrorPermissionViolation{
			{Type: "tenant", Subject: "im:message"},
		},
	}) {
		t.Fatal("expected permission gap to be recorded")
	}

	app.applyFeishuPermissionVerificationResult("app-1", nil, errors.New("scope list failed"))

	got := app.snapshotFeishuPermissionGaps("app-1")
	if len(got) != 1 {
		t.Fatalf("expected verification failure to keep gap, got %#v", got)
	}
	if got[0].LastVerified.IsZero() {
		t.Fatalf("expected verify timestamp to be recorded, got %#v", got[0])
	}
}

func TestApplyFeishuPermissionVerificationResultMatchesTokenTypeAndSatisfiers(t *testing.T) {
	for _, tc := range []struct {
		name, requiredScope, requiredType, grantedScope, grantedType string
		grantStatus                                                  int
		wantCleared                                                  bool
	}{
		{"user cannot clear tenant", "drive:drive", "tenant", "drive:drive", "user", 1, false},
		{"tenant cannot clear user", "drive:drive", "user", "drive:drive", "tenant", 1, false},
		{"ungranted cannot clear", "drive:drive", "tenant", "drive:drive", "tenant", 0, false},
		{"unknown type cannot clear", "drive:drive", "tenant", "drive:drive", "unknown", 1, false},
		{"satisfier clears", "im:chat:readonly", "tenant", "im:chat", "tenant", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := New(":0", ":0", &recordingGateway{}, serverIdentityForTest())
			app.observeFeishuPermissionError("app-1", &feishu.APIError{
				API: "drive.v1.file.list", Code: 99991672, Msg: "permission denied",
				PermissionViolations: []feishu.APIErrorPermissionViolation{{Type: tc.requiredType, Subject: tc.requiredScope}},
			})
			app.applyFeishuPermissionVerificationResult("app-1", []feishu.AppScopeStatus{{
				ScopeName: tc.grantedScope, ScopeType: tc.grantedType, GrantStatus: tc.grantStatus,
			}}, nil)
			if cleared := len(app.snapshotFeishuPermissionGaps("app-1")) == 0; cleared != tc.wantCleared {
				t.Fatalf("cleared = %t, want %t", cleared, tc.wantCleared)
			}
		})
	}
}

func TestVerifyFeishuPermissionGapsSkipsGrantQueryWithoutKnownGap(t *testing.T) {
	app := New(":0", ":0", &recordingGateway{}, serverIdentityForTest())
	previous := listFeishuAppGrantedScopes
	t.Cleanup(func() { listFeishuAppGrantedScopes = previous })
	listFeishuAppGrantedScopes = func(context.Context, feishu.LiveGatewayConfig) ([]feishu.AppScopeStatus, error) {
		t.Fatal("no gap should require no grant query")
		return nil, nil
	}
	app.verifyFeishuPermissionGaps(context.Background(), feishu.LiveGatewayConfig{GatewayID: "main"})
}

func TestApplyFeishuPermissionVerificationResultClearsBrokerPermissionBlocks(t *testing.T) {
	gateway := &permissionClearingGateway{}
	app := New(":0", ":0", gateway, serverIdentityForTest())
	if !app.observeFeishuPermissionError("app-1", &feishu.APIError{
		API:  "im.v1.message.create",
		Code: 99990001,
		Msg:  "permission denied",
		PermissionViolations: []feishu.APIErrorPermissionViolation{
			{Type: "tenant", Subject: "im:message"},
		},
	}) {
		t.Fatal("expected permission gap to be recorded")
	}
	app.applyFeishuPermissionVerificationResult("app-1", []feishu.AppScopeStatus{
		{ScopeName: "im:message", ScopeType: "tenant", GrantStatus: 1},
	}, nil)
	if len(gateway.clearCalls) != 1 {
		t.Fatalf("expected permission block clear to be forwarded once, got %#v", gateway.clearCalls)
	}
	if gateway.clearCalls[0].gatewayID != "app-1" {
		t.Fatalf("unexpected gateway id: %#v", gateway.clearCalls[0])
	}
	if len(gateway.clearCalls[0].scopes) != 1 || gateway.clearCalls[0].scopes[0].ScopeName != "im:message" {
		t.Fatalf("unexpected forwarded scopes: %#v", gateway.clearCalls[0].scopes)
	}
}

func TestPrimaryPermissionDecisionAcceptsGroupMessageScopes(t *testing.T) {
	for _, scope := range []string{"im:message.group_msg", "im:message.group_msg:readonly"} {
		decision := primaryPermissionDecisionFromScopes([]feishu.AppScopeStatus{
			{ScopeName: scope, ScopeType: "tenant", GrantStatus: 1},
		}, nil)
		if !decision.Allowed || decision.Scope != scope {
			t.Fatalf("scope %s decision = %#v, want allowed", scope, decision)
		}
	}
}

func TestPrimaryPermissionDecisionRejectsUserGroupMessageScope(t *testing.T) {
	decision := primaryPermissionDecisionFromScopes([]feishu.AppScopeStatus{
		{ScopeName: "im:message.group_msg", ScopeType: "user", GrantStatus: 1},
	}, nil)
	if decision.Allowed || decision.Reason != "missing_group_message_scope" {
		t.Fatalf("user-token scope decision = %#v, want missing tenant scope", decision)
	}
}

func TestPrimaryPermissionDecisionRejectsMissingScopeAndErrors(t *testing.T) {
	if decision := primaryPermissionDecisionFromScopes([]feishu.AppScopeStatus{
		{ScopeName: "im:message", ScopeType: "tenant", GrantStatus: 1},
	}, nil); decision.Allowed || decision.Reason != "missing_group_message_scope" {
		t.Fatalf("missing scope decision = %#v, want missing", decision)
	}
	if decision := primaryPermissionDecisionFromScopes(nil, errors.New("boom")); decision.Allowed || decision.Reason != "scope_read_failed" || decision.Err == nil {
		t.Fatalf("error decision = %#v, want failed with err", decision)
	}
}

func serverIdentityForTest() agentproto.ServerIdentity {
	return agentproto.ServerIdentity{
		PID:       42,
		StartedAt: time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC),
	}
}

type permissionClearingGateway struct {
	recordingGateway
	clearCalls           []permissionClearCall
	permissionResultHook func(string, string, error)
	permissionOwner      *feishu.LiveGatewayConfig
}

func (g *permissionClearingGateway) MatchesPermissionVerificationConfig(cfg feishu.LiveGatewayConfig) bool {
	return g.permissionOwner == nil || (g.permissionOwner.AppID == cfg.AppID && g.permissionOwner.AppSecret == cfg.AppSecret && g.permissionOwner.Domain == cfg.Domain)
}

func (g *permissionClearingGateway) SetPermissionResultHook(hook func(string, string, error)) {
	g.permissionResultHook = hook
}

func TestGatewayPermissionObserverRecordsBackgroundFailure(t *testing.T) {
	gateway := &permissionClearingGateway{}
	app := New(":0", ":0", gateway, serverIdentityForTest())
	if gateway.permissionResultHook == nil {
		t.Fatal("daemon did not register permission observer")
	}
	gateway.permissionResultHook("app-1", "drive.v1.file.list", &feishu.APIError{API: "drive.v1.file.list", Code: 99991672, Msg: "missing drive:drive"})
	if gaps := app.snapshotFeishuPermissionGaps("app-1"); len(gaps) != 1 || gaps[0].Scope != "drive:drive" {
		t.Fatalf("background permission gap = %#v", gaps)
	}
	gateway.permissionResultHook("other-app", "drive.v1.file.list", nil)
	gateway.permissionResultHook("app-1", "drive.v1.file.delete", nil)
	if len(app.snapshotFeishuPermissionGaps("app-1")) != 1 {
		t.Fatal("unrelated API success cleared gap")
	}
	gateway.permissionResultHook("app-1", "drive.v1.file.list", nil)
	if len(app.snapshotFeishuPermissionGaps("app-1")) != 0 {
		t.Fatal("actual API success did not clear its gap")
	}
}

type permissionClearCall struct {
	gatewayID string
	scopes    []feishu.AppScopeStatus
}

func (g *permissionClearingGateway) ClearGrantedPermissionBlocks(gatewayID string, scopes []feishu.AppScopeStatus) {
	g.clearCalls = append(g.clearCalls, permissionClearCall{
		gatewayID: gatewayID,
		scopes:    append([]feishu.AppScopeStatus(nil), scopes...),
	})
}
