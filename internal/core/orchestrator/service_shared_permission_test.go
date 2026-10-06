package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/YChange01/codex-feishu-link/internal/adapter/codex"
	"github.com/YChange01/codex-feishu-link/internal/core/agentproto"
	"github.com/YChange01/codex-feishu-link/internal/core/control"
	"github.com/YChange01/codex-feishu-link/internal/core/state"
)

func TestSharedPromptSettingsOnlyFreezeExplicitOverrides(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "explicit"}[explicit], func(t *testing.T) {
			now := time.Now()
			svc, sub := sharedSubscriptionFixture(t, &now)
			svc.ApplyAgentEvent("inst-1", sharedSubscriptionResult(sub, ""))
			inst, surface := svc.root.Instances["inst-1"], svc.root.Surfaces["surface-1"]
			thread := inst.Threads["thread-1"]
			thread.ExplicitModel, thread.ExplicitReasoningEffort = "desktop-model", "high"
			thread.ObservedAccessMode = agentproto.AccessModeConfirm
			want := state.ModelConfigRecord{}
			if explicit {
				want = state.ModelConfigRecord{Model: "chosen-model", ReasoningEffort: "low", AccessMode: agentproto.AccessModeConfirm}
				surface.PromptOverride = want
			}
			for _, threadID := range []string{"thread-1", ""} {
				got := svc.resolveFrozenPromptOverride(inst, surface, threadID, "/repo", state.ModelConfigRecord{})
				if got != want {
					t.Errorf("thread %q must only freeze explicit choices: got %#v want %#v", threadID, got, want)
				}
			}
			summary := svc.SurfaceSnapshot("surface-1").NextPrompt
			if !summary.UsesLocalRequestedOverrides || !summary.PlanModeUsesLocalRequested {
				t.Errorf("shared status must show native inheritance: %#v", summary)
			}
			if !explicit && (summary.EffectiveAccessMode != "" || summary.OverridePlanMode != "" || summary.EffectivePlanMode != "") {
				t.Errorf("native defaults must not be displayed as requested settings: %#v", summary)
			}
		})
	}
}

func TestSharedPermissionSettingsKeepUnmappedNativeTruth(t *testing.T) {
	now := time.Now()
	svc, sub := sharedSubscriptionFixture(t, &now)
	svc.ApplyAgentEvent("inst-1", sharedSubscriptionResult(sub, ""))
	tr := codex.NewTranslator("inst-1")
	for _, sandbox := range []string{"dangerFullAccess", "readOnly"} {
		result, err := tr.ObserveServer([]byte(`{"method":"thread/settings/updated","params":{"threadId":"thread-1","threadSettings":{"approvalPolicy":"never","sandboxPolicy":{"type":"` + sandbox + `"}}}}`))
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range result.Events {
			svc.ApplyAgentEvent("inst-1", event)
		}
	}
	summary := svc.SurfaceSnapshot("surface-1").NextPrompt
	if summary.ObservedThreadPermission == nil || summary.ObservedThreadPermission.ProjectionKind != agentproto.ObservedPermissionProjectionKindUnmapped || summary.ObservedThreadAccessMode != "" {
		t.Fatalf("read-only native permission must replace earlier full access: %#v", summary)
	}
	if display := control.ObservedThreadAccessDisplay(summary); !strings.Contains(display, "readOnly") || strings.Contains(display, "full") {
		t.Fatalf("status must preserve native permission without claiming full: %q", display)
	}
}

func TestSharedPermissionClearPreservesNativeOnDispatch(t *testing.T) {
	now := time.Now()
	svc, sub := sharedSubscriptionFixture(t, &now)
	svc.ApplyAgentEvent("inst-1", sharedSubscriptionResult(sub, ""))
	svc.ApplySurfaceAction(control.Action{Kind: control.ActionAccessCommand, SurfaceSessionID: "surface-1", Text: "/permission full"})
	svc.ApplySurfaceAction(control.Action{Kind: control.ActionAccessCommand, SurfaceSessionID: "surface-1", Text: "/permission clear"})
	events := svc.ApplySurfaceAction(control.Action{Kind: control.ActionTextMessage, SurfaceSessionID: "surface-1", MessageID: "prompt", Text: "continue"})
	send := findAgentCommand(events, agentproto.CommandPromptSend)
	if send == nil || send.Overrides != (agentproto.PromptOverrides{}) {
		t.Fatalf("clearing explicit permission must preserve native settings at dispatch: %#v", send)
	}
}

func TestSharedSettingNoticesDoNotInventFullAccess(t *testing.T) {
	for _, action := range []control.Action{
		{Kind: control.ActionModelCommand, Text: "/model gpt-6-astra"},
		{Kind: control.ActionReasoningCommand, Text: "/reasoning high"},
	} {
		t.Run(action.Text, func(t *testing.T) {
			now := time.Now()
			svc, sub := sharedSubscriptionFixture(t, &now)
			svc.ApplyAgentEvent("inst-1", sharedSubscriptionResult(sub, ""))
			svc.root.Instances["inst-1"].Threads["thread-1"].ObservedPermission = &agentproto.ObservedPermissionState{
				NativeMode: "approval=never; sandbox=read-only", ProjectionKind: agentproto.ObservedPermissionProjectionKindUnmapped,
			}
			action.SurfaceSessionID = "surface-1"
			for _, event := range svc.ApplySurfaceAction(action) {
				if event.Notice == nil || event.Notice.Code != "surface_override_updated" {
					continue
				}
				if strings.Contains(event.Notice.Text, "当前执行权限：full") || !strings.Contains(event.Notice.Text, "当前执行权限：不覆盖（跟随底层当前状态）") {
					t.Fatalf("setting notice invented a permission override: %s", event.Notice.Text)
				}
				return
			}
			t.Fatal("missing successful setting notice")
		})
	}
}

func TestOverrideNoticePermissionDefaultsAndExplicitChoices(t *testing.T) {
	for _, tc := range []struct {
		name    string
		summary control.PromptRouteSummary
		want    string
	}{
		{name: "independent default", want: "full"},
		{name: "shared explicit confirm", summary: control.PromptRouteSummary{UsesLocalRequestedOverrides: true, OverrideAccessMode: agentproto.AccessModeConfirm, EffectiveAccessMode: agentproto.AccessModeConfirm}, want: "confirm"},
		{name: "shared explicit full", summary: control.PromptRouteSummary{UsesLocalRequestedOverrides: true, OverrideAccessMode: agentproto.AccessModeFullAccess, EffectiveAccessMode: agentproto.AccessModeFullAccess}, want: "full"},
		{name: "shared observed full is not an override", summary: control.PromptRouteSummary{UsesLocalRequestedOverrides: true, EffectiveAccessMode: agentproto.AccessModeFullAccess}, want: "不覆盖（跟随底层当前状态）"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatOverrideNotice(tc.summary, "updated"); !strings.Contains(got, "当前执行权限："+tc.want+"\n") {
				t.Fatalf("wrong permission notice: %s", got)
			}
		})
	}
}
