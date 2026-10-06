package codex

import (
	"encoding/json"
	"strings"

	"github.com/YChange01/codex-feishu-link/internal/core/agentproto"
	"github.com/YChange01/codex-feishu-link/internal/xutil"
)

// Approval controls prompting, while sandbox controls access. Neither alone is
// evidence for a combined access preset, and an observation is never an override.
func observedCodexPermission(params map[string]any) *agentproto.ObservedPermissionState {
	approval := firstPermissionValue(params["approvalPolicy"], lookupAny(params, "config", "approval_policy"))
	sandbox := firstPermissionValue(params["sandboxPolicy"], params["sandbox"], lookupAny(params, "config", "sandbox_mode"), lookupAny(params, "config", "sandbox"))
	if approval == nil && sandbox == nil {
		return nil
	}
	permission := &agentproto.ObservedPermissionState{
		NativeMode:     "approval=" + permissionValueText(approval) + "; sandbox=" + permissionValueText(sandbox),
		ProjectionKind: agentproto.ObservedPermissionProjectionKindUnmapped,
	}
	sandboxType := xutil.LookupStringFromAny(sandbox)
	if policy, ok := sandbox.(map[string]any); ok {
		// Extra native restrictions cannot be represented by our coarse presets.
		if len(policy) != 1 {
			return permission
		}
		sandboxType = xutil.LookupStringFromAny(policy["type"])
	}
	switch xutil.LookupStringFromAny(approval) {
	case "never":
		if sandboxType == "danger-full-access" || sandboxType == "dangerFullAccess" {
			permission.ProjectedAccessMode = agentproto.AccessModeFullAccess
		}
	case "on-request":
		if sandboxType == "workspace-write" || sandboxType == "workspaceWrite" {
			permission.ProjectedAccessMode = agentproto.AccessModeConfirm
		}
	}
	if permission.ProjectedAccessMode != "" {
		permission.ProjectionKind = agentproto.ObservedPermissionProjectionKindExact
	}
	return permission
}

func firstPermissionValue(values ...any) any {
	for _, value := range values {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			continue
		}
		return value
	}
	return nil
}

func permissionValueText(value any) string {
	if value == nil {
		return "unknown"
	}
	if text, ok := value.(string); ok {
		return text
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "unknown"
	}
	return string(raw)
}
