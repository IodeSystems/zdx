package mcpcmd

import "github.com/modelcontextprotocol/go-sdk/mcp"

// RegisterOutlineTools registers structural outline tools (go_outline,
// ts_outline) scoped to the repo root. These are intended to be called by
// agents BEFORE read_file when probing what a file/package exposes — they
// return signatures and declarations without function bodies, so the payload
// is far smaller and avoids tool-result-size crashes in downstream chat
// templates.
func RegisterOutlineTools(srv *mcp.Server, root string) {
	registerGoOutline(srv, root)
	registerTSOutline(srv, root)
}
