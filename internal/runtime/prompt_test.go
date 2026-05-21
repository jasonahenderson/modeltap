package runtime

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

func capsWithTools(tools []protocol.ToolDefinition) *CapabilityManager {
	cm := NewCapabilityManager()
	cm.replaceTools(tools)
	return cm
}

func TestPrompt_Layer1_CoreBehavioralAlwaysPresent(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{})
	prompt, _ := pe.Assemble(NewCapabilityManager(), nil, protocol.ModeBuild)
	if !strings.Contains(prompt, "software-engineering assistant") {
		t.Errorf("Layer 1 missing from prompt:\n%s", prompt)
	}
}

func TestPrompt_Layer1_OverrideRespected(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{CoreBehavioral: "CUSTOM CORE"})
	prompt, _ := pe.Assemble(NewCapabilityManager(), nil, protocol.ModeBuild)
	if !strings.HasPrefix(prompt, "CUSTOM CORE") {
		t.Errorf("custom core override not honored: %s", prompt)
	}
}

func TestPrompt_Layer2_Tools(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{})
	caps := capsWithTools([]protocol.ToolDefinition{
		{Name: "read", Description: "Read a file", InputSchema: json.RawMessage(`{"type":"object"}`), RiskLevel: "read_only", OutputEnvelope: "text"},
	})
	prompt, _ := pe.Assemble(caps, nil, protocol.ModeBuild)
	if !strings.Contains(prompt, "## read") || !strings.Contains(prompt, "Read a file") {
		t.Errorf("Layer 2 missing tool details:\n%s", prompt)
	}
}

func TestPrompt_Layer2_NoToolsOmitted(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{})
	prompt, _ := pe.Assemble(NewCapabilityManager(), nil, protocol.ModeBuild)
	if strings.Contains(prompt, "## read") {
		t.Errorf("Layer 2 should be empty with no tools")
	}
}

func TestPrompt_Layer3_DomainConfig(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{DomainConfig: "DOMAIN INSTRUCTIONS HERE"})
	prompt, _ := pe.Assemble(NewCapabilityManager(), nil, protocol.ModeBuild)
	if !strings.Contains(prompt, "DOMAIN INSTRUCTIONS HERE") {
		t.Errorf("Layer 3 missing")
	}
}

func TestPrompt_Layer4_ProjectContext(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{})
	caps := NewCapabilityManager()
	caps.UpdateProjectContext(protocol.ProjectContext{
		Root:          "/repo",
		ConfigFile:    "AGENTS.md",
		ConfigContent: "be terse",
	})
	prompt, _ := pe.Assemble(caps, nil, protocol.ModeBuild)
	if !strings.Contains(prompt, "AGENTS.md") || !strings.Contains(prompt, "be terse") {
		t.Errorf("Layer 4 missing project context:\n%s", prompt)
	}
}

func TestPrompt_Layer5_PerMode(t *testing.T) {
	cases := map[protocol.Mode]string{
		protocol.ModePlan:  "PLAN mode",
		protocol.ModeBuild: "BUILD mode",
		protocol.ModeAuto:  "AUTO mode",
	}
	for mode, expect := range cases {
		t.Run(string(mode), func(t *testing.T) {
			pe := NewPromptEngine(PromptEngineOpts{})
			prompt, _ := pe.Assemble(NewCapabilityManager(), nil, mode)
			if !strings.Contains(prompt, expect) {
				t.Errorf("mode %q missing %q in prompt", mode, expect)
			}
		})
	}
}

func TestPrompt_Layer7_ModelOverride(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{})
	session := &ActiveSession{ModelOverride: "claude-haiku-4-5"}
	prompt, _ := pe.Assemble(NewCapabilityManager(), session, protocol.ModeBuild)
	if !strings.Contains(prompt, "claude-haiku-4-5") {
		t.Errorf("override notice missing:\n%s", prompt)
	}
}

func TestPrompt_TokenCounting_ApproximatesContent(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{})
	prompt, tokens := pe.Assemble(NewCapabilityManager(), nil, protocol.ModeBuild)
	// Crude bound: estimate ~ chars/4. Allow ±50% for joiners.
	expectMin := len(prompt) / 8
	expectMax := len(prompt) / 2
	if tokens < expectMin || tokens > expectMax {
		t.Errorf("tokens=%d outside [%d, %d] for prompt length %d", tokens, expectMin, expectMax, len(prompt))
	}
}

func TestPrompt_Trimming_DropsLayers6and7Only(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{
		WindowSize:        200, // tiny window
		SystemPromptShare: 0.5, // 100 token budget
	})
	caps := capsWithTools([]protocol.ToolDefinition{
		{Name: "read", Description: strings.Repeat("very long description ", 20), InputSchema: json.RawMessage(`{"type":"object"}`), RiskLevel: "read_only", OutputEnvelope: "text"},
	})
	caps.UpdateProjectContext(protocol.ProjectContext{
		ConfigFile:    "AGENTS.md",
		ConfigContent: strings.Repeat("project guidance ", 30),
	})
	session := &ActiveSession{ModelOverride: "claude-opus-4-6"}
	prompt, _ := pe.Assemble(caps, session, protocol.ModeBuild)

	// The prompt should still contain the pinned tool name even after trim.
	if !strings.Contains(prompt, "## read") {
		t.Errorf("pinned tool layer missing after trim")
	}
	// The override notice (Layer 7) should be trimmed.
	if strings.Contains(prompt, "claude-opus-4-6") {
		t.Errorf("Layer 7 should have been trimmed but is present")
	}
}

func TestPrompt_PerTurnReassembly_RespectsCurrentMode(t *testing.T) {
	pe := NewPromptEngine(PromptEngineOpts{})
	caps := NewCapabilityManager()
	planPrompt, _ := pe.Assemble(caps, nil, protocol.ModePlan)
	buildPrompt, _ := pe.Assemble(caps, nil, protocol.ModeBuild)
	if planPrompt == buildPrompt {
		t.Errorf("prompt did not change when mode changed")
	}
}
