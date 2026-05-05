package harness

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jasonahenderson/modeltap/internal/harness/tools"
)

// MCPTool bridges a tool discovered on an MCP server into the harness
// tools.Tool interface, forwarding Execute through an MCPClient's
// CallTool. The tool's Name is namespaced with the server id ("mcp/<server>:<tool>")
// so the registry's unique-name invariant survives multiple MCP
// servers that happen to expose same-named tools.
type MCPTool struct {
	namespace   string
	rawName     string // the server-side name (used in tools/call)
	displayName string // fully-qualified name used in the harness registry
	description string
	schema      json.RawMessage
	client      *MCPClient
}

// NewMCPTool builds an MCPTool from an MCP descriptor + its serving
// client. namespace is the server id ("file-tools") which becomes
// part of the registered name ("mcp/file-tools:read_file"). An empty
// namespace falls back to "mcp".
func NewMCPTool(namespace string, desc MCPToolDescriptor, client *MCPClient) *MCPTool {
	if namespace == "" {
		namespace = "mcp"
	}
	return &MCPTool{
		namespace:   namespace,
		rawName:     desc.Name,
		displayName: "mcp/" + namespace + ":" + desc.Name,
		description: desc.Description,
		schema:      desc.InputSchema,
		client:      client,
	}
}

func (m *MCPTool) Name() string        { return m.displayName }
func (m *MCPTool) Description() string { return m.description }
func (m *MCPTool) InputSchema() json.RawMessage {
	if len(m.schema) == 0 {
		return json.RawMessage(`{"type":"object"}`)
	}
	return m.schema
}
func (m *MCPTool) OutputEnvelope() string { return "text" }

// RiskLevel is always RiskExecute for remote tools — the harness
// can't audit what the remote server does, so every call prompts in
// non-autonomous modes (the permission layer already handles this
// based on risk level).
func (m *MCPTool) RiskLevel() tools.RiskLevel { return tools.RiskExecute }

// Execute forwards the invocation through the MCP client and
// collapses the returned content blocks into a single text result.
// isError → StatusError result. Binary content (images, resources)
// gets a base64 tag-line so the output has SOMETHING meaningful even
// when the block isn't plain text.
func (m *MCPTool) Execute(ctx context.Context, input json.RawMessage) (*tools.ToolExecResult, error) {
	if m.client == nil {
		return tools.ErrorResult("mcp client detached"), nil
	}
	resp, err := m.client.CallTool(ctx, m.rawName, input)
	if err != nil {
		return tools.ErrorResult("%v", err), nil
	}
	text := flattenMCPContent(resp.Content)
	if resp.IsError {
		return tools.ErrorResult("%s", text), nil
	}
	return tools.SuccessResult(text, "text"), nil
}

// flattenMCPContent joins text blocks verbatim and annotates non-text
// blocks with their MIME type + size. Order is preserved. Empty
// content arrays produce "(no content)".
func flattenMCPContent(blocks []MCPContentBlock) string {
	if len(blocks) == 0 {
		return "(no content)"
	}
	var b strings.Builder
	for i, blk := range blocks {
		if i > 0 {
			b.WriteString("\n")
		}
		switch blk.Type {
		case "text":
			b.WriteString(blk.Text)
		case "image":
			mime := blk.MimeType
			if mime == "" {
				mime = "image/unknown"
			}
			b.WriteString("[image: " + mime + ", " + approxSize(len(blk.Data)) + " base64]")
		case "resource":
			b.WriteString("[resource: " + nonEmpty(blk.MimeType, "unknown") + "]")
		default:
			b.WriteString("[" + blk.Type + " block]")
		}
	}
	return b.String()
}

// approxSize reports a rough human-readable size for a base64 payload.
// Keeps the MCPTool output compact instead of dumping raw binary.
func approxSize(n int) string {
	switch {
	case n >= 1<<20:
		return formatSize(n, 1<<20, "MB")
	case n >= 1<<10:
		return formatSize(n, 1<<10, "KB")
	}
	return formatSize(n, 1, "B")
}

func formatSize(n, unit int, suffix string) string {
	// Avoid pulling strconv / fmt just for a size formatter — the
	// handful of digits fits fine in a manual itoa.
	v := n / unit
	return formatInt64(int64(v)) + " " + suffix
}

func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
