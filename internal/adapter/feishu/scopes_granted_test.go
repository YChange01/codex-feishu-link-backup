package feishu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestListAppGrantedScopesUsesEffectiveGrantsWithoutConfigFallback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "effective grants", true: "query failure"}[fail], func(t *testing.T) {
			configHits := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/open-apis/auth/v3/tenant_access_token/internal":
					_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"fake-token"}`))
				case "/open-apis/application/v6/scopes":
					if fail {
						_, _ = w.Write([]byte(`{"code":99991672,"msg":"access denied"}`))
						return
					}
					_, _ = w.Write([]byte(`{"code":0,"data":{"scopes":[{"scope_name":"drive:drive","scope_type":"tenant","grant_status":2},{"scope_name":"drive:drive","scope_type":"user","grant_status":1}]}}`))
				default:
					configHits++
					_, _ = w.Write([]byte(`{"code":0,"data":{"app":{"scopes":[{"scope":"drive:drive","token_types":["tenant"]}]}}}`))
				}
			}))
			defer server.Close()
			scopes, err := ListAppGrantedScopes(context.Background(), LiveGatewayConfig{GatewayID: "main", AppID: server.URL, AppSecret: "fake-secret", Domain: server.URL})
			if configHits != 0 {
				t.Fatalf("runtime query consulted application config %d times", configHits)
			}
			if fail {
				if err == nil || len(scopes) != 0 {
					t.Fatalf("failure must preserve error without grants: scopes=%#v err=%v", scopes, err)
				}
				return
			}
			want := []AppScopeStatus{{ScopeName: "drive:drive", ScopeType: "tenant", GrantStatus: 2}, {ScopeName: "drive:drive", ScopeType: "user", GrantStatus: 1}}
			if err != nil || !reflect.DeepEqual(scopes, want) {
				t.Fatalf("grants = %#v, %v; want %#v", scopes, err, want)
			}
		})
	}
}
