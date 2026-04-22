package bff

import (
	"fmt"
	"strings"

	"github.com/jasonahenderson/modeltap/internal/protocol"
	"github.com/jasonahenderson/modeltap/internal/provider"
)

// defaultCoreBehavioral is the bundled Layer-1 core behavioral text.
// FEAT-0008 anticipates loading this from internal/bff/assets/; for v1
// we ship an inline default string so the engine works without asset
// embedding. Operators can override via PromptEngineOpts.CoreBehavioral.
const defaultCoreBehavioral = `You are a software-engineering assistant integrated into a developer harness.

Operate by the following principles:
- Be concise and direct. Prefer code over prose where it answers the question.
- Read project context (config, conventions, existing code) before proposing changes.
- Respect the user's stated mode: in PLAN mode, do not modify files or run commands; in BUILD mode, execute the request directly; in AUTO mode, decide based on complexity.
- When using tools, call them with valid JSON inputs that match each tool's schema. Do not invent tools that were not registered.
- When you encounter an obstacle, report it briefly with the smallest reproduction needed to act on it.`

// PromptLayer is one slice of the assembled system prompt. Number is
// the FEAT-0008 layer ordinal (1..7); Pinned layers are never trimmed.
type PromptLayer struct {
	Number  int
	Name    string
	Content string
	Tokens  int
	Pinned  bool
}

// PromptEngineOpts configures a PromptEngine instance.
type PromptEngineOpts struct {
	CoreBehavioral    string  // overrides defaultCoreBehavioral when non-empty
	DomainConfig      string  // Layer 3 — operator-supplied
	WindowSize        int     // total context window for the resolved model
	SystemPromptShare float64 // fraction of WindowSize allocated to the system prompt; default 0.25
}

// PromptEngine assembles the system prompt from the seven FEAT-0008
// layers. It is owned per-connection (or per-server, when stateless)
// and reads dynamic state from the supplied CapabilityManager and
// ActiveSession on every Assemble call.
type PromptEngine struct {
	coreBehavioral    string
	domainConfig      string
	windowSize        int
	systemPromptShare float64
}

// NewPromptEngine constructs an engine with the given options.
func NewPromptEngine(opts PromptEngineOpts) *PromptEngine {
	core := opts.CoreBehavioral
	if core == "" {
		core = defaultCoreBehavioral
	}
	share := opts.SystemPromptShare
	if share <= 0 {
		share = 0.25
	}
	return &PromptEngine{
		coreBehavioral:    core,
		domainConfig:      opts.DomainConfig,
		windowSize:        opts.WindowSize,
		systemPromptShare: share,
	}
}

// SetWindowSize updates the window budget. Useful when the resolved
// model changes between turns.
func (pe *PromptEngine) SetWindowSize(n int) { pe.windowSize = n }

// modeInstructions maps each Mode to its Layer-5 text.
var modeInstructions = map[protocol.Mode]string{
	protocol.ModePlan:  "You are in PLAN mode. Analyze the request and propose a plan. Do NOT make file modifications, execute commands, or take actions. Only read files and describe what you would do.",
	protocol.ModeBuild: "You are in BUILD mode. Execute the user's request directly. Make file changes, run commands, and take actions as needed.",
	protocol.ModeAuto:  "You are in AUTO mode. Decide whether to plan or execute based on the complexity of the request. For simple tasks, execute directly. For complex tasks, propose a plan first.",
}

// AssembleLayers1to5 builds the first five layers from caps + mode.
// Caller (Assemble) appends Layers 6-7 from session state.
func (pe *PromptEngine) AssembleLayers1to5(caps *CapabilityManager, mode protocol.Mode) []PromptLayer {
	out := make([]PromptLayer, 0, 5)

	// Layer 1 — core behavioral (pinned).
	out = append(out, PromptLayer{
		Number:  1,
		Name:    "core_behavioral",
		Content: pe.coreBehavioral,
		Tokens:  provider.EstimateTokens(pe.coreBehavioral),
		Pinned:  true,
	})

	// Layer 2 — tool-use instructions (pinned).
	if caps != nil {
		if tools := caps.Tools(); len(tools) > 0 {
			content := pe.toolUseInstructions(tools)
			out = append(out, PromptLayer{
				Number:  2,
				Name:    "tool_use",
				Content: content,
				Tokens:  provider.EstimateTokens(content),
				Pinned:  true,
			})
		}
	}

	// Layer 3 — domain instructions (pinned per FEAT-0008).
	if pe.domainConfig != "" {
		out = append(out, PromptLayer{
			Number:  3,
			Name:    "domain_instructions",
			Content: pe.domainConfig,
			Tokens:  provider.EstimateTokens(pe.domainConfig),
			Pinned:  true,
		})
	}

	// Layer 4 — project context (pinned per FEAT-0008).
	if caps != nil {
		if pc := caps.ProjectContext(); pc.ConfigContent != "" {
			content := fmt.Sprintf("## Project Instructions (%s)\n%s", pc.ConfigFile, pc.ConfigContent)
			out = append(out, PromptLayer{
				Number:  4,
				Name:    "project_context",
				Content: content,
				Tokens:  provider.EstimateTokens(content),
				Pinned:  true,
			})
		}
	}

	// Layer 5 — mode-specific instructions (pinned).
	if instr, ok := modeInstructions[mode]; ok {
		out = append(out, PromptLayer{
			Number:  5,
			Name:    "mode",
			Content: instr,
			Tokens:  provider.EstimateTokens(instr),
			Pinned:  true,
		})
	}

	return out
}

// toolUseInstructions formats the registered tool catalog into the
// Layer-2 prompt block. Per design D3.3, each tool gets its name,
// description, and input schema.
func (pe *PromptEngine) toolUseInstructions(tools []protocol.ToolDefinition) string {
	var sb strings.Builder
	sb.WriteString("You have access to the following tools:\n\n")
	for _, t := range tools {
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\nInput schema:\n```json\n%s\n```\n\n",
			t.Name, t.Description, string(t.InputSchema)))
	}
	return sb.String()
}

// knowledgeLayer returns the Layer-6 knowledge injection. FEAT-0011
// stub for now — returns "".
func (pe *PromptEngine) knowledgeLayer() string { return "" }

// sessionStateLayer composes Layer 7 from the session's volatile
// state: pinned items, compaction summaries, and any active model
// override. Returns "" when nothing is worth including.
func (pe *PromptEngine) sessionStateLayer(session *ActiveSession) string {
	if session == nil {
		return ""
	}
	var parts []string

	// Compaction summaries / pinned items would come from extended
	// ActiveSession fields populated by WU-061 (compaction). For now
	// expose only the model-override notice — the other slots are
	// reserved for that WU.
	if session.ModelOverride != "" {
		parts = append(parts, fmt.Sprintf("## Model Override\nUser has overridden routing to: %s", session.ModelOverride))
	}
	return strings.Join(parts, "\n\n")
}

// Assemble builds the complete system prompt from all 7 layers,
// applying the trim policy when the total exceeds budget.
// Returns the assembled string and its estimated token count.
func (pe *PromptEngine) Assemble(caps *CapabilityManager, session *ActiveSession, mode protocol.Mode) (string, int) {
	layers := pe.AssembleLayers1to5(caps, mode)

	if k := pe.knowledgeLayer(); k != "" {
		layers = append(layers, PromptLayer{
			Number: 6, Name: "knowledge", Content: k, Tokens: provider.EstimateTokens(k),
		})
	}
	if s := pe.sessionStateLayer(session); s != "" {
		layers = append(layers, PromptLayer{
			Number: 7, Name: "session_state", Content: s, Tokens: provider.EstimateTokens(s),
		})
	}

	if pe.windowSize > 0 {
		budget := int(float64(pe.windowSize) * pe.systemPromptShare)
		if sumLayerTokens(layers) > budget {
			layers = trimLayers(layers, budget)
		}
	}
	return joinLayers(layers), sumLayerTokens(layers)
}

// trimLayers drops Layer 6 first, then Layer 7, until total ≤ budget.
// Layers 1-5 are pinned and never trimmed (FEAT-0008 contract).
func trimLayers(layers []PromptLayer, budget int) []PromptLayer {
	for _, num := range []int{6, 7} {
		if sumLayerTokens(layers) <= budget {
			return layers
		}
		for i := range layers {
			if layers[i].Number == num && !layers[i].Pinned {
				layers[i].Content = ""
				layers[i].Tokens = 0
			}
		}
	}
	return layers
}

func sumLayerTokens(layers []PromptLayer) int {
	total := 0
	for _, l := range layers {
		total += l.Tokens
	}
	return total
}

func joinLayers(layers []PromptLayer) string {
	var sb strings.Builder
	for i, l := range layers {
		if l.Content == "" {
			continue
		}
		if sb.Len() > 0 && i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(l.Content)
	}
	return sb.String()
}
