package feishu

import (
	gatewaypkg "github.com/YChange01/codex-feishu-link/internal/adapter/feishu/gateway"
	previewpkg "github.com/YChange01/codex-feishu-link/internal/adapter/feishu/preview"
)

// MatchesPermissionVerificationConfig checks the active owner as well as the
// persisted configuration checked by the daemon. Config may be pending apply.
func (c *MultiGatewayController) MatchesPermissionVerificationConfig(cfg LiveGatewayConfig) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	worker := c.workers[gatewaypkg.NormalizeGatewayID(cfg.GatewayID)]
	return worker != nil && worker.runtime != nil && worker.config.AppID == cfg.AppID &&
		worker.config.AppSecret == cfg.AppSecret && worker.config.Domain == cfg.Domain
}

type gatewayPermissionBroker interface {
	permissionBroker() *FeishuCallBroker
}

func (g *LiveGateway) permissionBroker() *FeishuCallBroker { return g.broker }

// SetPermissionResultHook reports permission failures and successful API calls
// without making foreground callers or background maintenance own gap state.
func (c *MultiGatewayController) SetPermissionResultHook(hook func(string, string, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.permissionResultHook = hook
	for gatewayID, worker := range c.workers {
		if worker != nil {
			c.configurePermissionObserverLocked(gatewayID, worker.generation, worker.runtime)
		}
	}
}

func (c *MultiGatewayController) configurePermissionObserverLocked(gatewayID string, generation uint64, runtime gatewayRuntime) {
	owner, ok := runtime.(gatewayPermissionBroker)
	if !ok || owner.permissionBroker() == nil {
		return
	}
	owner.permissionBroker().setPermissionResultHook(func(api string, err error) {
		c.mu.RLock()
		worker := c.workers[gatewayID]
		hook := c.permissionResultHook
		current := worker != nil && worker.runtime == runtime && worker.generation == generation
		var gate *gatewayActionGate
		if current && hook != nil {
			gate = worker.actionGate
			current = gate.begin()
		}
		c.mu.RUnlock()
		if current && hook != nil {
			defer gate.end()
			hook(gatewayID, api, err)
		}
	})
}

func newGatewayDrivePreviewAPI(gatewayID string, runtime gatewayRuntime) previewpkg.DriveAPI {
	if owner, ok := runtime.(gatewayPermissionBroker); ok && owner.permissionBroker() != nil {
		return &larkDrivePreviewAPI{client: runtime.Client(), broker: owner.permissionBroker()}
	}
	return NewLarkDrivePreviewAPI(gatewayID, runtime.Client())
}

func (b *FeishuCallBroker) setPermissionResultHook(hook func(string, error)) {
	b.mu.Lock()
	b.permissionResultHook = hook
	b.mu.Unlock()
}

func (b *FeishuCallBroker) observePermissionResult(api string, err error) {
	b.mu.Lock()
	hook := b.permissionResultHook
	b.mu.Unlock()
	if hook != nil {
		hook(api, err)
	}
}
