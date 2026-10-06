package codex

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/YChange01/codex-feishu-link/internal/core/agentproto"
)

func TestSharedNewThreadInheritsNativePermissionDefaults(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			tr := NewTranslator("inst-1")
			tr.SetSharedAppServerMode(true)
			tr.latestThreadStartParams = map[string]any{"model": "stale-model", "approvalPolicy": "never", "sandbox": "danger-full-access"}
			tr.newThreadTurnTemplate = map[string]any{"model": "stale-model", "approvalPolicy": "never", "sandboxPolicy": map[string]any{"type": "dangerFullAccess"}}
			command := agentproto.Command{Kind: agentproto.CommandPromptSend, CommandID: "new", Target: agentproto.Target{CWD: "/repo"}}
			if explicit {
				command.Overrides = agentproto.PromptOverrides{AccessMode: agentproto.AccessModeConfirm}
			}
			frames, err := tr.TranslateCommand(command)
			if err != nil {
				t.Fatal(err)
			}
			start := sharedSubscriptionOnlyFrame(t, frames, "thread/start")
			if explicit {
				if start.Params["approvalPolicy"] != "on-request" || start.Params["sandbox"] != "workspace-write" {
					t.Errorf("explicit permission lost: %#v", start.Params)
				}
			} else if start.Params["approvalPolicy"] != nil || start.Params["sandbox"] != nil {
				t.Errorf("new shared thread overrode native permission: %#v", start.Params)
			}
			if start.Params["model"] != nil {
				t.Errorf("new shared thread replayed stale model: %#v", start.Params)
			}
			created := sharedSubscriptionObserve(t, tr, fmt.Sprintf(`{"id":%q,"result":{"thread":{"id":"new-thread","cwd":"/repo"}}}`, start.ID))
			turn := sharedSubscriptionOnlyFrame(t, created.OutboundToCodex, "turn/start")
			if explicit {
				if turn.Params["approvalPolicy"] != "on-request" || lookupString(turn.Params, "sandboxPolicy", "type") != "workspaceWrite" {
					t.Errorf("explicit turn permission lost: %#v", turn.Params)
				}
			} else if turn.Params["approvalPolicy"] != nil || turn.Params["sandboxPolicy"] != nil {
				t.Errorf("first shared turn replayed permission: %#v", turn.Params)
			}
			if turn.Params["model"] != nil {
				t.Errorf("first shared turn replayed stale model: %#v", turn.Params)
			}
		})
	}
}

func TestCodexObservedPermissionRequiresExactPolicyPair(t *testing.T) {
	for _, tc := range []struct{ approval, sandbox, access string }{
		{"never", "dangerFullAccess", agentproto.AccessModeFullAccess},
		{"on-request", "workspaceWrite", agentproto.AccessModeConfirm},
		{"never", "readOnly", ""},
		{"never", "workspaceWrite", ""},
		{"on-request", "readOnly", ""},
		{"", "dangerFullAccess", ""},
		{"never", "", ""},
	} {
		t.Run(tc.approval+"_"+tc.sandbox, func(t *testing.T) {
			tr := NewTranslator("inst-1")
			params := map[string]any{"threadId": "thread-1", "approvalPolicy": tc.approval, "sandboxPolicy": map[string]any{"type": tc.sandbox}}
			raw, _ := json.Marshal(map[string]any{"method": "turn/start", "params": params})
			result, err := tr.ObserveClient(raw)
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range result.Events {
				if event.Kind != agentproto.EventConfigObserved || event.ObservedPermission == nil {
					continue
				}
				if event.AccessMode != tc.access || event.ObservedPermission.ProjectedAccessMode != tc.access {
					t.Fatalf("coarse permission fabricated from incomplete/mixed pair: %#v", event)
				}
				if tc.access == "" && event.ObservedPermission.ProjectionKind != agentproto.ObservedPermissionProjectionKindUnmapped {
					t.Fatalf("non-exact pair must be unmapped: %#v", event.ObservedPermission)
				}
				return
			}
			t.Fatal("missing native permission observation")
		})
	}
}

func TestSharedSubscriptionObservesNativePermissionWithoutOverride(t *testing.T) {
	tr := NewTranslator("inst-1")
	tr.SetSharedAppServerMode(true)
	resume := sharedSubscriptionBegin(t, tr, "subscribe", "thread-1")
	result := sharedSubscriptionObserve(t, tr, fmt.Sprintf(`{"id":%q,"result":{"thread":{"id":"thread-1","cwd":"/repo"},"approvalPolicy":"never","sandbox":{"type":"readOnly"}}}`, resume.ID))
	for _, event := range result.Events {
		if event.Kind == agentproto.EventConfigObserved && event.ObservedPermission != nil && event.ObservedPermission.ProjectionKind == agentproto.ObservedPermissionProjectionKindUnmapped && event.AccessMode == "" {
			return
		}
	}
	t.Fatalf("subscription dropped native permission evidence: %#v", result.Events)
}

func TestSharedCreatedThreadObservesNativePermission(t *testing.T) {
	tr := NewTranslator("inst-1")
	tr.SetSharedAppServerMode(true)
	frames, err := tr.TranslateCommand(agentproto.Command{Kind: agentproto.CommandPromptSend, CommandID: "new", Target: agentproto.Target{CWD: "/repo"}})
	if err != nil {
		t.Fatal(err)
	}
	start := sharedSubscriptionOnlyFrame(t, frames, "thread/start")
	result := sharedSubscriptionObserve(t, tr, fmt.Sprintf(`{"id":%q,"result":{"thread":{"id":"new-thread","cwd":"/repo"},"approvalPolicy":"never","sandbox":{"type":"readOnly"}}}`, start.ID))
	for _, event := range result.Events {
		if event.Kind == agentproto.EventThreadDiscovered && event.ObservedPermission != nil && event.ObservedPermission.ProjectionKind == agentproto.ObservedPermissionProjectionKindUnmapped && event.AccessMode == "" {
			return
		}
	}
	t.Fatalf("created thread dropped native permission evidence: %#v", result.Events)
}

func TestSharedExistingThreadPermissionOverridesAreExplicit(t *testing.T) {
	for _, access := range []string{"", agentproto.AccessModeConfirm, agentproto.AccessModeFullAccess} {
		t.Run("access="+access, func(t *testing.T) {
			tr := NewTranslator("inst-1")
			tr.SetSharedAppServerMode(true)
			sharedSubscriptionReady(t, tr, "sub", "thread-1", `[]`)
			tr.turnStartByThread["thread-1"] = map[string]any{"approvalPolicy": "never", "sandboxPolicy": map[string]any{"type": "readOnly"}}
			frames, err := tr.TranslateCommand(agentproto.Command{Kind: agentproto.CommandPromptSend, Target: agentproto.Target{ThreadID: "thread-1"}, Overrides: agentproto.PromptOverrides{AccessMode: access}})
			if err != nil {
				t.Fatal(err)
			}
			turn := sharedSubscriptionOnlyFrame(t, frames, "turn/start")
			if access == "" {
				for _, key := range []string{"approvalPolicy", "sandboxPolicy", "model", "effort", "collaborationMode"} {
					if _, exists := turn.Params[key]; exists {
						t.Fatalf("unrequested setting %s must be omitted: %#v", key, turn.Params)
					}
				}
			} else if turn.Params["approvalPolicy"] != agentproto.ApprovalPolicyForAccessMode(access) || lookupString(turn.Params, "sandboxPolicy", "type") != agentproto.TurnSandboxPolicyForAccessMode(access)["type"] {
				t.Fatalf("explicit permission was lost: %#v", turn.Params)
			}
		})
	}
}

func TestCodexObservedCustomSandboxCannotClaimExactPreset(t *testing.T) {
	permission := observedCodexPermission(map[string]any{"approvalPolicy": "on-request", "sandboxPolicy": map[string]any{"type": "workspaceWrite", "networkAccess": false, "writableRoots": []any{"/specific"}}})
	if permission == nil || permission.ProjectionKind != agentproto.ObservedPermissionProjectionKindUnmapped || permission.ProjectedAccessMode != "" {
		t.Fatalf("custom sandbox must retain its native restrictions: %#v", permission)
	}
}
