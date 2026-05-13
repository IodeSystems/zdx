package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iodesystems/zdx-go/internal/llm"
)

// localDispatcher wraps an MCP client session — in-memory (newLocalDispatcher,
// the in-process tools the provider registered itself) or stdio-spawned
// (newRemoteDispatcher, dx-agent --mcp-stdio inside a dev container). The
// dispatch surface is identical; the chat loop never knows which transport
// it's talking through.
//
// persona gates which tools the agent is allowed to invoke (IS-1097): the
// IS-1090 design matrix lives in CheckToolPermission and is enforced here at
// the dispatch boundary, before the tool call crosses the wire. Empty
// persona normalizes to DefaultPersona inside CheckToolPermission so older
// callsites that don't supply one degrade to the dev-tier permission set.
type localDispatcher struct {
	serverSession *mcp.ServerSession // nil for remote dispatchers (no in-process server)
	clientSession *mcp.ClientSession
	tools         []*mcp.Tool
	persona       string
}

// newLocalDispatcher wires an in-memory MCP client/server pair and lists the
// registered tools. The caller owns the server and must keep it alive.
//
// persona is plumbed through so Dispatch can apply the IS-1097 permission
// matrix without the chat loop having to filter tool_calls itself.
func newLocalDispatcher(ctx context.Context, srv *mcp.Server, persona string) (*localDispatcher, error) {
	t1, t2 := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, t1, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp server connect: %w", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "dx-agent-local-client", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		ss.Close()
		return nil, fmt.Errorf("mcp client connect: %w", err)
	}

	list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		cs.Close()
		ss.Close()
		return nil, fmt.Errorf("mcp list tools: %w", err)
	}

	return &localDispatcher{
		serverSession: ss,
		clientSession: cs,
		tools:         list.Tools,
		persona:       persona,
	}, nil
}

// newRemoteDispatcher spawns command as a subprocess (typically
// `docker exec -i <c> dx-agent --mcp-stdio`) and connects an MCP client to
// its stdin/stdout. The host process runs the LLM loop; tool calls cross
// the boundary into the subprocess where dx-agent --mcp-stdio executes
// them against /workspace inside the container.
//
// persona is enforced host-side before tool calls cross the wire, so the
// remote subprocess never sees banned tool calls regardless of transport.
//
// The subprocess's stderr is forwarded to the host's stderr so panics and
// permission errors are visible during dev. Closing the dispatcher closes
// stdin → SIGTERM → SIGKILL via mcp.CommandTransport's Close ladder.
func newRemoteDispatcher(ctx context.Context, command []string, persona string) (*localDispatcher, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("remote dispatcher: command must not be empty")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stderr = os.Stderr

	client := mcp.NewClient(&mcp.Implementation{Name: "dx-agent-remote-client", Version: "0.1.0"}, nil)
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect remote MCP %v: %w", command, err)
	}
	list, err := cs.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		_ = cs.Close()
		return nil, fmt.Errorf("list tools on remote MCP: %w", err)
	}
	return &localDispatcher{
		clientSession: cs,
		tools:         list.Tools,
		persona:       persona,
	}, nil
}

func (d *localDispatcher) Close() {
	if d.clientSession != nil {
		_ = d.clientSession.Close()
	}
	if d.serverSession != nil {
		_ = d.serverSession.Close()
	}
}

// toolNames returns the names of every tool visible to the dispatcher. Used
// by setup-event instrumentation so operators can see the agent's tool
// surface at session start without trawling the JSONL.
func (d *localDispatcher) toolNames() []string {
	names := make([]string, 0, len(d.tools))
	for _, t := range d.tools {
		names = append(names, t.Name)
	}
	return names
}

// OpenAIFunctions returns the registered tools in OpenAI function-calling format.
func (d *localDispatcher) OpenAIFunctions() []llm.ToolDef {
	out := make([]llm.ToolDef, 0, len(d.tools))
	for _, t := range d.tools {
		params := t.InputSchema
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, llm.ToolDef{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return out
}

// Dispatch invokes a tool by name with JSON-encoded arguments and returns a
// stringified result suitable for feeding back as a tool message.
//
// Enforces the IS-1097 persona/tool permission matrix before the call
// crosses the wire: banned tool/persona combinations short-circuit with a
// structured isError=true result so the LLM observes the denial as a normal
// tool failure and can route a question to the suggested tier.
func (d *localDispatcher) Dispatch(ctx context.Context, name, argsJSON string) (string, bool, error) {
	if decision := CheckToolPermission(d.persona, name, argsJSON); !decision.Allowed {
		persona := d.persona
		if persona == "" {
			persona = DefaultPersona
		}
		return FormatToolPermissionError(persona, name, decision), true, nil
	}
	var args any
	if argsJSON == "" {
		args = map[string]any{}
	} else {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", true, fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	res, err := d.clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return err.Error(), true, nil
	}
	isErr := res.IsError
	text := renderToolResult(res)
	// Hard cap on tool-result payload to keep us under crash thresholds in
	// downstream chat templates (observed: llama.cpp jinja aborts the server
	// when a tool message exceeds ~50KB on Qwen3 templates). Per-tool caps in
	// the registered tools are first-line defense; this is the safety net for
	// any tool that slips through.
	const maxToolResultBytes = 48 * 1024
	if len(text) > maxToolResultBytes {
		text = text[:maxToolResultBytes] + "\n…[truncated by dispatcher: tool result exceeded 48KB]"
	}
	return text, isErr, nil
}

func renderToolResult(res *mcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	if res.StructuredContent != nil {
		b, err := json.Marshal(res.StructuredContent)
		if err == nil {
			return string(b)
		}
	}
	var out []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			out = append(out, tc.Text)
		}
	}
	if len(out) == 0 {
		return ""
	}
	joined := ""
	for i, s := range out {
		if i > 0 {
			joined += "\n"
		}
		joined += s
	}
	return joined
}
