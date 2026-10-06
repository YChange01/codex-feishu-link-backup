package feishu

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPermissionVerificationRequiresMatchingActiveOwner(t *testing.T) {
	controller := NewMultiGatewayController()
	query := LiveGatewayConfig{GatewayID: "main", AppID: "app", AppSecret: "secret", Domain: "https://open.feishu.cn"}
	worker := &gatewayWorker{runtime: &LiveGateway{}, config: GatewayAppConfig{GatewayID: query.GatewayID, AppID: query.AppID, AppSecret: query.AppSecret, Domain: query.Domain}}
	controller.workers["main"] = worker
	if !controller.MatchesPermissionVerificationConfig(query) {
		t.Fatal("matching active owner rejected")
	}
	for _, mismatch := range []LiveGatewayConfig{
		{GatewayID: "other", AppID: query.AppID, AppSecret: query.AppSecret, Domain: query.Domain},
		{GatewayID: "main", AppID: "other-app", AppSecret: query.AppSecret, Domain: query.Domain},
		{GatewayID: "main", AppID: query.AppID, AppSecret: "rotated-secret", Domain: query.Domain},
		{GatewayID: "main", AppID: query.AppID, AppSecret: query.AppSecret, Domain: "https://open.larksuite.com"},
	} {
		if controller.MatchesPermissionVerificationConfig(mismatch) {
			t.Fatal("mismatched verification owner accepted")
		}
	}
	worker.runtime = nil
	if controller.MatchesPermissionVerificationConfig(query) {
		t.Fatal("inactive worker accepted verification evidence")
	}
}

func TestPermissionResultRejectsRemovedAndReaddedRuntimeWithSameGeneration(t *testing.T) {
	controller := NewMultiGatewayController()
	old := &LiveGateway{broker: NewFeishuCallBroker("main", nil)}
	controller.workers["main"] = &gatewayWorker{runtime: old, generation: 1, actionGate: newGatewayActionGate()}
	reports := 0
	hook := func(string, string, error) { reports++ }
	controller.SetPermissionResultHook(hook)
	if err := controller.RemoveApp(context.Background(), "main"); err != nil {
		t.Fatal(err)
	}
	if err := controller.UpsertApp(context.Background(), GatewayAppConfig{GatewayID: "main"}); err != nil {
		t.Fatal(err)
	}
	current := &LiveGateway{broker: NewFeishuCallBroker("main", nil)}
	worker := controller.workers["main"]
	worker.runtime, worker.generation, worker.actionGate = current, 1, newGatewayActionGate()
	controller.SetPermissionResultHook(hook)
	old.broker.observePermissionResult("drive.v1.file.list", errors.New("old permission denial"))
	old.broker.observePermissionResult("drive.v1.file.list", nil)
	if reports != 0 {
		t.Fatalf("retired runtime reused generation and emitted %d results", reports)
	}
	current.broker.observePermissionResult("drive.v1.file.list", nil)
	if reports != 1 {
		t.Fatalf("current runtime result lost, reports=%d", reports)
	}
}

func TestRemoveAppDrainsAdmittedPermissionResultOutsideControllerLock(t *testing.T) {
	controller := NewMultiGatewayController()
	runtime := &LiveGateway{broker: NewFeishuCallBroker("main", nil)}
	retired, entered, release, reported, removed := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	controller.workers["main"] = &gatewayWorker{runtime: runtime, generation: 1, actionGate: newGatewayActionGate(), cancel: func() { close(retired) }}
	controller.SetPermissionResultHook(func(string, string, error) {
		controller.Status()
		close(entered)
		<-release
	})
	go func() { runtime.broker.observePermissionResult("drive.v1.file.list", nil); close(reported) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("result callback blocked on controller lock")
	}
	go func() { _ = controller.RemoveApp(context.Background(), "main"); close(removed) }()
	select {
	case <-retired:
	case <-time.After(time.Second):
		t.Fatal("worker did not enter retirement")
	}
	select {
	case <-removed:
		t.Error("removal finished before admitted result drained")
	default:
	}
	close(release)
	select {
	case <-reported:
	case <-time.After(time.Second):
		t.Fatal("callback did not finish")
	}
	select {
	case <-removed:
	case <-time.After(time.Second):
		t.Fatal("retirement did not finish after callback drained")
	}
	// Both success and failure after retirement must be rejected.
	runtime.broker.observePermissionResult("drive.v1.file.list", nil)
	runtime.broker.observePermissionResult("drive.v1.file.list", errors.New("late denial"))
}
