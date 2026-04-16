# Pre-Review: Tool Framework + Tools Bundle (WU-075–079)

**Design doc:** `docs/history/2026-04-16-design-tool-framework-075-076-077-078-079.md`
**Reviewed against:**
- `docs/features/0009-terminal-harness.md` (FEAT-0009)
- `docs/releases/v0.2.0/track-b-terminal-harness.md` (Track B)
- `internal/protocol/messages.go` (WU-039 protocol types)

---

## Blocking

### B1. Tool.RiskLevel() returns `RiskLevel` (custom type) but ToolDefinition.RiskLevel is `string`

The design defines `RiskLevel` as a Go `string` type with four constants (`read_only`, `write`, `execute`, `destructive`). The protocol `ToolDefinition.RiskLevel` field is a plain `string`. The `Registry.All()` method must convert `Tool.RiskLevel()` to the wire-form string for `ToolDefinition`. This is not explicitly addressed — if the conversion is forgotten or the constant values drift, tools will register with wrong risk levels. The design should document the `All()` method's conversion logic, and tests should assert that every built-in tool's `RiskLevel()` value matches one of the four wire-legal strings.

### B2. ToolExecResult.ToProtocol() must populate ToolResult.ToolCallID but has no source for it

`ToolExecResult` (D2.4) has no `ToolCallID` field. The `ToProtocol(toolCallID string)` method takes it as a parameter, which is fine — but `Executor.Execute()` (D2.3) returns `*ToolExecResult` without a tool call ID. The caller must pair the result with the original tool call's ID, but the design does not show where the tool call ID flows from the protocol layer (server sends a `tool.call` event with an ID) through the executor and back into the `ToolResult`. This wiring gap between protocol events and the executor needs to be documented to avoid losing the correlation ID.

### B3. Missing `MethodHistoryList` in protocol constants — WU-092 dependency

WU-092 (BFF-sourced command history) sends `history.list` to the server. There is no `MethodHistoryList` constant in `messages.go` and no `HistoryList` request type. This is a Track A (WU-091) deliverable, not a Track B design flaw per se, but the tool framework design's "Dependencies Consumed" section does not mention WU-091/WU-092, and Track B's WU-092 depends on this type existing. Flag for cross-track coordination.

---

## Attention

### A1. FEAT-0009 lists 13 separate tool rows; design collapses Read* into a single ReadTool

FEAT-0009's tool table has five separate rows: Read, ReadPDF, ReadDOCX, ReadImage, ReadSpreadsheet. The design (correctly, per FEAT-0009 paragraph below the table) unifies these behind a single `Read` tool with format auto-detection. This is the right call and FEAT-0009 itself says "the harness detects the file type and applies the appropriate extraction." However, the tool count in the design ("all 13 built-in tools") still references 13, yet the registry only has 9 distinct tool names (Read, Write, Edit, Bash, Git, Glob, Grep, WebSearch, WebFetch). Either the count should be updated to 9, or the design should clarify that "13" counts logical capabilities, not registered tool names. Inconsistent counts will confuse implementation.

### A2. Git tool's dynamic RiskLevel not reflected in Tool interface

The `Tool` interface has a single `RiskLevel()` method that returns a fixed level. But the Git tool's risk level varies per invocation: `read_only` for `git status`, `execute` for `git commit`, `destructive` for `git push --force`. The permission matrix (D3.2) accounts for this, but the interface does not. `PermissionEnforcer.Check(tool Tool, input json.RawMessage)` receives the input, so it can inspect the git subcommand — but this means the permission enforcer must contain Git-specific parsing logic rather than relying on the tool's declared risk level. The design should explicitly state that Git (and Bash) are special cases where the enforcer inspects input rather than using `RiskLevel()`, or the interface should support `RiskLevelFor(input json.RawMessage) RiskLevel`.

### A3. Bash tool's dynamic RiskLevel has the same problem as Git

Same issue as A2. Bash's `RiskLevel()` must return a single value, but the actual risk varies from `execute` (safe commands) to `destructive` (dangerous commands). The design's `IsDangerous()` function handles this at the executor level, but `Registry.All()` will register Bash with a single fixed risk level in the `ToolDefinition`. The server sees one risk level; the harness enforces a different, input-dependent one. This is acceptable if intentional (the server trusts the harness for permission enforcement per FEAT-0009: "Permissions are enforced entirely in the harness"), but should be documented as a design choice.

### A4. WebFetch per-domain approval tracked in PermissionEnforcer but Approve/ApproveDomain split is unclear in executor flow

The permission matrix says WebFetch uses "Prompt/domain" in Default and Accept-Edits modes. `PermissionEnforcer` has both `approved map[string]bool` (tool-level) and `domains map[string]bool` (domain-level). But the executor flow (D2.3 step 2) only describes `Check` → allow/prompt/deny, with no special handling for domain extraction from WebFetch input. The `Check` method must parse the URL from WebFetch's input JSON to extract the domain — this parsing logic should be documented.

### A5. Executor.tracker field declared but NewExecutor signature does not accept it

`Executor` struct has a `tracker *FileTracker` field, but `NewExecutor(registry, permissions)` takes only two arguments. Either the tracker should be a third parameter, or it should be created internally. Since Edit and Write both need the tracker, and the tracker must be shared across tools, the constructor needs updating.

### A6. No vision-capability gating for image reads

FEAT-0009 states: "For images, the harness returns a base64-encoded representation if the current model supports vision, or an error explaining the model does not support images." The design's ReadTool always returns base64 for images with no check of the current model's capabilities. The tool has no access to model metadata. Either the executor or a higher layer needs to gate image results, or ReadTool needs a capabilities provider injected.

### A7. SSRF check is incomplete — DNS rebinding not covered

D14 lists private IP ranges and schemes for SSRF prevention. However, it does not address DNS rebinding: a domain like `evil.com` can resolve to `127.0.0.1`. The `isPrivateAddress` function checks the URL's host string, not the resolved IP. The security review (WU-094) is noted as downstream, but the design should at least flag this as a known limitation or specify that resolution-time checking is required.

### A8. No `pages` parameter for PDF reads

FEAT-0009 does not mandate it, but PDF files can be very large. The Read tool's input schema has `offset` and `limit` (line-based, for text). For PDFs, there is no page-range parameter. Large PDFs will produce enormous text output. Consider adding a `pages` parameter or documenting the truncation strategy.

---

## Nit

### N1. `OutputEnvelope` naming inconsistency

The `Tool` interface method is `OutputEnvelope() string` and `ToolDefinition` has `OutputEnvelope string`. The `ToolExecResult` calls the same concept `OutputType string`. Pick one name. The protocol wire field is `output_type` (in `ToolResult`) vs. `output_envelope` (in `ToolDefinition`). These are arguably different things (the definition's envelope vs. the result's actual type), but the design's D2.4 says `OutputType` maps to `"text", "json", "binary", "image"` — the same values as `OutputEnvelope`. Aligning the names would reduce confusion.

### N2. `replace_all` default not in JSON schema

D8 says `replace_all` defaults to false, but the JSON schema snippet does not include `"default": false`. Minor — the Go implementation will handle it — but the schema should be explicit for model consumption.

### N3. Grep `output_mode` default not in schema

D12 says default is `"files_with_matches"` but the schema does not declare a default. Same as N2.

### N4. DangerousPatterns regex for `rm` is overly specific

The pattern `rm\s+(-[a-zA-Z]*r[a-zA-Z]*f|(-[a-zA-Z]*f[a-zA-Z]*r))\b` catches `rm -rf` and `rm -fr` but not `rm -r -f` (separate flags). This is a known pattern-matching limitation. Not blocking since the executor already prompts for all Bash commands in Default mode, but worth noting for the autonomous case.

### N5. `BuiltinOptions.SearchEngine` values undocumented in schema

D2.2 says `"brave"` or `"serpapi"` but this is only in a code comment. If the harness config surfaces this, it should be validated.

### N6. CSV listed as `FormatCSV` but handled under ReadTool, not ReadSpreadsheet

D6.1 declares `FormatCSV` as distinct from `FormatSpreadsheet`, and D6.2 has a separate `readCSV` function. This is correct but the FEAT-0009 tool table groups CSV under "ReadSpreadsheet (XLSX/CSV)". Minor naming mismatch — implementation is fine, just document the grouping.
