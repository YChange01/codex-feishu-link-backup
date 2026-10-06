package feishu

import (
	"context"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

func TestFeishuCallBrokerReportsOnlyVerifiedSDKSuccess(t *testing.T) {
	broker := NewFeishuCallBroker("main", nil)
	reports := 0
	broker.setPermissionResultHook(func(string, error) { reports++ })
	for _, result := range []any{
		nil, (*larkdrive.ListFileResp)(nil),
		&larkdrive.ListFileResp{CodeError: larkcore.CodeError{Code: 99991672}},
	} {
		_, err := broker.do(context.Background(), CallSpec{API: "drive.v1.file.list"}, func(context.Context) (any, error) { return result, nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	if reports != 0 {
		t.Fatal("nil Go error reported success without successful SDK result")
	}
	_, _ = broker.do(context.Background(), CallSpec{API: "drive.v1.file.list"}, func(context.Context) (any, error) {
		return &larkdrive.ListFileResp{CodeError: larkcore.CodeError{Code: 0}}, nil
	})
	if reports != 1 {
		t.Fatalf("verified API success reports=%d, want 1", reports)
	}
}

func TestFeishuCallBrokerGrantMatchesPermissionRequirement(t *testing.T) {
	for _, tc := range []struct {
		name, requiredScope, requiredType, grantedScope, grantedType string
		grantStatus                                                  int
		wantCleared                                                  bool
	}{
		{"unknown gap defaults tenant", "drive:drive", "", "drive:drive", "user", 1, false},
		{"user cannot clear tenant", "drive:drive", "tenant", "drive:drive", "user", 1, false},
		{"ungranted cannot clear", "drive:drive", "tenant", "drive:drive", "tenant", 0, false},
		{"satisfier clears", "im:chat:readonly", "tenant", "im:chat", "tenant", 1, true},
		{"unknown granted type rejected", "drive:drive", "tenant", "drive:drive", "unknown", 1, false},
		{"tenant grant restores unknown gap", "drive:drive", "", "drive:drive", "tenant", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broker := NewFeishuCallBroker("app-1", nil)
			spec := CallSpec{API: "drive.v1.file.list"}
			broker.markPermissionBlocked(spec, PermissionGapEvidence{Scope: tc.requiredScope, ScopeType: tc.requiredType}, nil)
			broker.ClearGrantedPermissionBlocks([]AppScopeStatus{{ScopeName: tc.grantedScope, ScopeType: tc.grantedType, GrantStatus: tc.grantStatus}})
			err, _ := broker.currentPermissionBlock(spec)
			if cleared := err == nil; cleared != tc.wantCleared {
				t.Fatalf("cleared = %t, want %t", cleared, tc.wantCleared)
			}
		})
	}
}
