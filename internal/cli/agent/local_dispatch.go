package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iodesystems/zdx-go/internal/llm"
)

// localDispatcher connects an in-process MCP client to a server registered
// with dx project + filesystem + shell tools. It exposes an OpenAI-compatible
// tools list and dispatches tool_call invocations from the model.
type localDispatcher struct {
	serverSession *mcp.ServerSession
	clientSession *mcp.ClientSession
	tools         []*mcp.Tool
}

// newLocalDispatcher wires an in-memory MCP client/server pair and lists the
// registered tools. The caller owns the server and must keep it alive.
func newLocalDispatcher(ctx context.Context, srv *mcp.Server) (*localDispatcher, error) {
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
func (d *localDispatcher) Dispatch(ctx context.Context, name, argsJSON string) (string, bool, error) {
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
