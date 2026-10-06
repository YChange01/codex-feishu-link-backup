package codex

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/YChange01/codex-feishu-link/internal/core/agentproto"
	"github.com/YChange01/codex-feishu-link/internal/core/jsonrpcutil"
)

type sharedSubscriptionRequest struct {
	Command    agentproto.Command
	Generation uint64
	ReadTurns  bool
	Revision   uint64
}

func (t *Translator) SetSharedAppServerMode(enabled bool) {
	t.sharedAppServer = enabled
}

// ObserveServer scopes shared-daemon notifications to this connection's selected
// thread. Other clients' threads must never acquire this proxy's active-turn slot.
func (t *Translator) ObserveServer(raw []byte) (Result, error) {
	result, err := t.observeServer(raw)
	if err != nil || !t.sharedAppServer {
		return result, err
	}
	filtered := result.Events[:0]
	for _, event := range result.Events {
		if event.ThreadID != "" && event.ThreadID != t.sharedThreadID && event.Kind != agentproto.EventThreadHistoryRead {
			continue
		}
		if event.Kind == agentproto.EventTurnStarted || event.Kind == agentproto.EventTurnCompleted {
			t.sharedTurnRevision++
		}
		turnScoped := event.TurnID != "" || event.Kind == agentproto.EventRequestResolved
		if turnScoped && event.Kind != agentproto.EventThreadSubscribed && event.Kind != agentproto.EventThreadHistoryRead && t.pendingRemoteTurnByThread[event.ThreadID] != "" {
			t.sharedPendingTurnEvents = append(t.sharedPendingTurnEvents, event)
			continue
		}
		filtered = append(filtered, event)
	}
	result.Events = filtered
	return result, nil
}

func (t *Translator) translateSharedThreadSubscribe(command agentproto.Command) ([][]byte, error) {
	threadID := strings.TrimSpace(command.Target.ThreadID)
	if !t.sharedAppServer || threadID == "" {
		return nil, fmt.Errorf("thread.subscribe requires shared app-server mode and a thread id")
	}
	var frames [][]byte
	if previous := t.sharedThreadID; previous != "" && previous != threadID {
		requestID := t.NextRequest("thread-unsubscribe")
		t.pendingSuppressedResponse[requestID] = suppressedResponseContext{Action: "thread/unsubscribe"}
		frame, err := sharedSubscriptionFrame(requestID, "thread/unsubscribe", map[string]any{"threadId": previous})
		if err != nil {
			return nil, err
		}
		frames = append(frames, frame)
	}
	t.sharedThreadID = threadID
	t.sharedSubscriptionReady = false
	t.sharedSubscriptionGeneration++
	t.sharedSubscriptionRequests = map[string]sharedSubscriptionRequest{}
	requestID := t.NextRequest("thread-subscribe")
	t.sharedSubscriptionRequests[requestID] = sharedSubscriptionRequest{Command: command, Generation: t.sharedSubscriptionGeneration}
	// Omit cwd/model/approval overrides: subscription observes the existing
	// desktop thread and must not replace its execution configuration.
	frame, err := sharedSubscriptionFrame(requestID, "thread/resume", map[string]any{"threadId": threadID, "excludeTurns": true})
	if err != nil {
		return nil, err
	}
	return append(frames, frame), nil
}

func sharedSubscriptionFrame(id, method string, params map[string]any) ([]byte, error) {
	raw, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	return append(raw, '\n'), err
}

func (t *Translator) adoptSharedCreatedThread(threadID string, command agentproto.Command, message map[string]any) (Result, error) {
	if !t.sharedAppServer {
		return Result{}, nil
	}
	var frames [][]byte
	if previous := t.sharedThreadID; previous != "" && previous != threadID {
		id := t.NextRequest("thread-unsubscribe")
		t.pendingSuppressedResponse[id] = suppressedResponseContext{Action: "thread/unsubscribe"}
		frame, err := sharedSubscriptionFrame(id, "thread/unsubscribe", map[string]any{"threadId": previous})
		if err != nil {
			return Result{}, err
		}
		frames = append(frames, frame)
	}
	t.sharedThreadID, t.sharedSubscriptionReady = threadID, true
	t.sharedSubscriptionGeneration++
	t.sharedSubscriptionRequests = map[string]sharedSubscriptionRequest{}
	record := parseThreadRecord(message["result"])
	discovered := buildThreadDiscoveredEvent(record, threadID, record.CWD, record.Name, "", true, record.RuntimeStatus)
	discovered.ObservedPermission = observedCodexPermission(lookupMap(message, "result"))
	if discovered.ObservedPermission != nil {
		discovered.AccessMode = discovered.ObservedPermission.ProjectedAccessMode
	}
	return Result{OutboundToCodex: frames, Events: []agentproto.Event{
		{Kind: agentproto.EventThreadSubscribed, ThreadID: threadID, CommandID: command.CommandID, Status: "created"},
		discovered,
	}}, nil
}

func (t *Translator) observeSharedSubscriptionResponse(requestID string, message map[string]any) (Result, bool) {
	pending, ok := t.sharedSubscriptionRequests[requestID]
	if !ok {
		return Result{}, false
	}
	delete(t.sharedSubscriptionRequests, requestID)
	threadID := pending.Command.Target.ThreadID
	if pending.Generation != t.sharedSubscriptionGeneration || threadID != t.sharedThreadID {
		return Result{Suppress: true}, true
	}
	event := agentproto.Event{Kind: agentproto.EventThreadSubscribed, ThreadID: threadID, CommandID: pending.Command.CommandID}
	if problem := jsonrpcutil.ExtractErrorMessage(message); problem != "" {
		event.Status, event.ErrorMessage = "failed", problem
		return Result{Suppress: true, Events: []agentproto.Event{event}}, true
	}
	if !pending.ReadTurns {
		record := parseThreadRecord(message["result"])
		if record.ThreadID != threadID {
			event.Status, event.ErrorMessage = "failed", "Codex resumed a different or missing thread"
			return Result{Suppress: true, Events: []agentproto.Event{event}}, true
		}
		t.currentThreadID = threadID
		t.knownThreadCWD[threadID] = record.CWD
		// Resume responses expose effective configuration beside `thread` on
		// some app-server versions. It is response evidence, not our request.
		if model := lookupString(message, "result", "model"); model != "" {
			record.Model = model
		}
		if effort := lookupString(message, "result", "reasoningEffort"); effort != "" {
			record.ReasoningEffort = effort
		}
		t.mergeObservedThread(threadID, record.ModelProviderID, record.Model, record.ReasoningEffort)
		id := t.NextRequest("thread-subscribe-turns")
		pending.ReadTurns, pending.Revision = true, t.sharedTurnRevision
		t.sharedSubscriptionRequests[id] = pending
		frame, err := sharedSubscriptionFrame(id, "thread/turns/list", map[string]any{
			"threadId": threadID, "limit": 2, "sortDirection": "desc", "itemsView": "notLoaded",
		})
		if err != nil {
			event.Status, event.ErrorMessage = "failed", err.Error()
			return Result{Suppress: true, Events: []agentproto.Event{event}}, true
		}
		var events []agentproto.Event
		if params := lookupMap(message, "result"); observedCodexPermission(params) != nil {
			events = configObservedEvents(threadID, record.CWD, params, false)
		}
		return Result{Suppress: true, OutboundToCodex: [][]byte{frame}, Events: events}, true
	}
	turns, ok := lookupAny(message, "result", "data").([]any)
	if !ok {
		event.Status, event.ErrorMessage = "failed", "Codex thread/turns/list response has no data array"
		return Result{Suppress: true, Events: []agentproto.Event{event}}, true
	}
	event.Status = "ready"
	observed := t.observedThreads[threadID]
	event.Model, event.ReasoningEffort = observed.Model, observed.ReasoningEffort
	if pending.Revision != t.sharedTurnRevision {
		event.Metadata = map[string]any{"turnSnapshotStale": true}
	} else {
		for _, raw := range turns {
			turn, ok := raw.(map[string]any)
			if !ok || !strings.EqualFold(lookupString(turn, "status"), "inProgress") {
				continue
			}
			event.TurnID = lookupString(turn, "id")
			if event.TurnID == "" {
				event.Status, event.ErrorMessage = "failed", "active Codex turn has no id"
				return Result{Suppress: true, Events: []agentproto.Event{event}}, true
			}
			if t.turnInitiators[event.TurnID].Kind == "" {
				t.turnInitiators[event.TurnID] = agentproto.Initiator{Kind: agentproto.InitiatorLocalUI}
			}
			break
		}
	}
	t.sharedSubscriptionReady = true
	return Result{Suppress: true, Events: []agentproto.Event{event}}, true
}
