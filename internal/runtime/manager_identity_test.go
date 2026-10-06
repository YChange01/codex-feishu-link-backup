package relayruntime

import (
	"testing"

	"github.com/YChange01/codex-feishu-link/internal/core/agentproto"
)

func TestManagerClassifiesWelcomeAcrossProductRename(t *testing.T) {
	manager := NewManager(ManagerConfig{Identity: testBinaryIdentity(), Paths: testPaths(t)})
	for _, tc := range []struct {
		name        string
		productName string
		fingerprint string
		want        ProbeStatus
	}{
		{"current same build", "codex-feishu-link", "fp-1", ProbeCompatible},
		{"legacy same build", "codex-feishu-relay", "fp-1", ProbeCompatible},
		{"current different build", "codex-feishu-link", "fp-2", ProbeIncompatible},
		{"legacy different build", "codex-feishu-relay", "fp-2", ProbeIncompatible},
		{"unrelated service", "another-product", "fp-1", ProbeUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := manager.classifyWelcome(agentproto.Welcome{
				Protocol: agentproto.WireProtocol,
				Server: &agentproto.ServerIdentity{
					BinaryIdentity: agentprotoIdentity(tc.productName, "1.0.0", tc.fingerprint),
				},
			})
			if got.Status != tc.want {
				t.Fatalf("classifyWelcome = %s (%v), want %s", got.Status, got.Err, tc.want)
			}
		})
	}
}
