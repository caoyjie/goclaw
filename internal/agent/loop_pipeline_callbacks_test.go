package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/mediaremote"
	"github.com/nextlevelbuilder/goclaw/internal/pipeline"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

type finalThinkingStreamProvider struct{}

func (p finalThinkingStreamProvider) Chat(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "final", Thinking: "non-stream thinking"}, nil
}

func (p finalThinkingStreamProvider) ChatStream(context.Context, providers.ChatRequest, func(providers.StreamChunk)) (*providers.ChatResponse, error) {
	return &providers.ChatResponse{Content: "final", Thinking: "final streamed thinking"}, nil
}

func (p finalThinkingStreamProvider) DefaultModel() string { return "test-model" }
func (p finalThinkingStreamProvider) Name() string         { return "test-provider" }

func TestMakeCallLLM_StreamsFinalThinkingWhenNoThinkingChunkArrives(t *testing.T) {
	col := &eventCollector{}
	loop := &Loop{id: "test-agent", onEvent: col.onEvent}
	req := &RunRequest{
		RunID:      "run-1",
		SessionKey: "sess-1",
		Channel:    "telegram",
		Stream:     true,
	}
	state := &pipeline.RunState{
		Provider:  finalThinkingStreamProvider{},
		Model:     "test-model",
		Iteration: 0,
	}

	resp, err := loop.makeCallLLM(req, col.onEvent)(context.Background(), state, providers.ChatRequest{})
	if err != nil {
		t.Fatalf("makeCallLLM returned error: %v", err)
	}
	if resp == nil || resp.Thinking != "final streamed thinking" {
		t.Fatalf("stream response = %+v, want final thinking preserved", resp)
	}

	thinking := col.filter(protocol.ChatEventThinking)
	if len(thinking) != 1 {
		t.Fatalf("thinking events = %+v, want exactly one final thinking event", thinking)
	}
	payload, ok := thinking[0].Payload.(map[string]string)
	if !ok || payload["content"] != "final streamed thinking" {
		t.Fatalf("thinking payload = %+v", thinking[0].Payload)
	}
}

type fakeRemoteMediaProcessor struct {
	err    error
	inputs []mediaremote.ToolResultInput
}

func (f *fakeRemoteMediaProcessor) ProcessToolResult(ctx context.Context, input mediaremote.ToolResultInput) ([]mediaremote.RemoteMediaRef, error) {
	f.inputs = append(f.inputs, input)
	if f.err != nil {
		return nil, f.err
	}
	return []mediaremote.RemoteMediaRef{{
		URL:      "https://media.example.com/a.png",
		Key:      "goclaw-media/a.png",
		MimeType: "image/png",
		Kind:     "image",
		Size:     123,
	}}, nil
}

func TestProcessToolResultCapturesRemoteMedia(t *testing.T) {
	processor := &fakeRemoteMediaProcessor{}
	tenantID := uuid.Must(uuid.NewV7())
	loop := &Loop{id: "ceo-assistant", tenantID: tenantID, remoteMedia: processor}
	rs := &runState{}
	state := &pipeline.RunState{Input: &pipeline.RunInput{SessionKey: "session-secret"}}
	req := &RunRequest{RunID: "run-1", SessionKey: "session-secret"}

	_, _, action := loop.processToolResult(
		context.Background(),
		rs,
		req,
		func(AgentEvent) {},
		providers.ToolCall{ID: "call-1", Name: "mcp_comfy_cloud_get_output", Arguments: map[string]any{"prompt": "cat"}},
		"mcp_comfy_cloud_get_output",
		&tools.Result{ForLLM: `{"url":"https://comfy.example.com/out.png","mime_type":"image/png"}`},
		false,
	)
	syncBridgeToState(rs, state, action)

	if len(processor.inputs) != 1 {
		t.Fatalf("processor calls = %d, want 1", len(processor.inputs))
	}
	if processor.inputs[0].TenantID != tenantID.String() || processor.inputs[0].AgentID != "ceo-assistant" {
		t.Fatalf("input identity = %+v", processor.inputs[0])
	}
	if processor.inputs[0].SessionKeyHash == "" || processor.inputs[0].SessionKeyHash == "session-secret" {
		t.Fatalf("session hash = %q, should be non-empty hash", processor.inputs[0].SessionKeyHash)
	}
	if len(state.Tool.RemoteMediaResults) != 1 {
		t.Fatalf("remote media results = %d, want 1", len(state.Tool.RemoteMediaResults))
	}
	if state.Tool.RemoteMediaResults[0].URL != "https://media.example.com/a.png" {
		t.Fatalf("remote media URL = %q", state.Tool.RemoteMediaResults[0].URL)
	}
}

func TestProcessToolResultRemoteMediaErrorDoesNotFailTool(t *testing.T) {
	processor := &fakeRemoteMediaProcessor{err: errors.New("upload failed")}
	loop := &Loop{id: "ceo-assistant", tenantID: uuid.Must(uuid.NewV7()), remoteMedia: processor}
	rs := &runState{}
	state := &pipeline.RunState{Input: &pipeline.RunInput{SessionKey: "session-secret"}}
	req := &RunRequest{RunID: "run-1", SessionKey: "session-secret"}

	toolMsg, _, action := loop.processToolResult(
		context.Background(),
		rs,
		req,
		func(AgentEvent) {},
		providers.ToolCall{ID: "call-1", Name: "mcp_comfy_cloud_get_output"},
		"mcp_comfy_cloud_get_output",
		&tools.Result{ForLLM: `{"url":"https://comfy.example.com/out.png","mime_type":"image/png"}`},
		false,
	)
	syncBridgeToState(rs, state, action)

	if toolMsg.IsError {
		t.Fatal("remote media failure should not mark tool message as error")
	}
	if len(state.Tool.RemoteMediaResults) != 0 {
		t.Fatalf("remote media results = %d, want 0", len(state.Tool.RemoteMediaResults))
	}
}
