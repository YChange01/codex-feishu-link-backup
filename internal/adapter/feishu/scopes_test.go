package feishu

import "testing"

func TestMatchScopeRequirementKeepsTokenTypesDistinct(t *testing.T) {
	for _, tc := range []struct {
		name, requiredType, grantedType string
		want                            bool
	}{
		{"tenant rejects user", "tenant", "user", false},
		{"user rejects tenant", "user", "tenant", false},
		{"unknown grant rejected", "tenant", "unknown", false},
		{"unknown requirement rejected", "unknown", "tenant", false},
		{"user token alias", "user", "user_access_token", true},
		{"absent type defaults tenant", "", "tenant", true},
		{"absent type rejects user", "", "user", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got := MatchScopeRequirement("drive:drive", tc.requiredType, []AppScopeStatus{{ScopeName: "drive:drive", ScopeType: tc.grantedType, GrantStatus: 1}})
			if got != tc.want {
				t.Fatalf("match = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestMatchScopeRequirementTreatsChatAsChatReadonlySatisfier(t *testing.T) {
	scope, ok := MatchScopeRequirement("im:chat:readonly", "tenant", []AppScopeStatus{
		{ScopeName: "im:chat", ScopeType: "tenant", GrantStatus: 1},
	})
	if !ok || scope != "im:chat" {
		t.Fatalf("MatchScopeRequirement = %q, %v; want im:chat satisfier", scope, ok)
	}
}
