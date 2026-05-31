// Package capability maps models to the features they support, so the
// dispatcher never silently falls back to a model that cannot honor the
// request (e.g. routing a tool-calling request to a model without tools, or a
// vision request to a text-only model).
//
// The matrix is heuristic: it matches model ids by substring against a set of
// known families. Unknown models are assumed to support the baseline set
// (text + streaming) only, which is the safe conservative default.
package capability

import (
	"strings"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// rule associates a model-id substring with the capabilities that family has.
type rule struct {
	match string
	caps  []core.Capability
}

// rules are evaluated in order; all matching rules union their capabilities.
// Substrings are lowercased before matching.
var rules = []rule{
	// Frontier chat models: full feature set.
	{"gpt-4", []core.Capability{core.CapToolCalling, core.CapVision, core.CapStructuredOutput, core.CapLongContext}},
	{"gpt-5", []core.Capability{core.CapToolCalling, core.CapVision, core.CapReasoning, core.CapStructuredOutput, core.CapLongContext}},
	{"o1", []core.Capability{core.CapToolCalling, core.CapReasoning, core.CapStructuredOutput}},
	{"o3", []core.Capability{core.CapToolCalling, core.CapReasoning, core.CapStructuredOutput}},
	{"claude", []core.Capability{core.CapToolCalling, core.CapVision, core.CapReasoning, core.CapLongContext}},
	{"gemini", []core.Capability{core.CapToolCalling, core.CapVision, core.CapAudioInput, core.CapLongContext}},
	{"deepseek", []core.Capability{core.CapToolCalling, core.CapReasoning}},
	{"glm", []core.Capability{core.CapToolCalling, core.CapLongContext}},
	{"minimax", []core.Capability{core.CapToolCalling, core.CapLongContext}},
	{"qwen", []core.Capability{core.CapToolCalling}},
	{"kimi", []core.Capability{core.CapToolCalling, core.CapLongContext}},
	{"grok", []core.Capability{core.CapToolCalling, core.CapVision}},
	{"llama", []core.Capability{core.CapToolCalling}},
	{"mistral", []core.Capability{core.CapToolCalling}},
}

// baseline is granted to every model.
var baseline = []core.Capability{core.CapStreaming}

// Of returns the capability set for a model id.
func Of(model string) core.CapabilitySet {
	set := core.NewCapabilitySet(baseline...)
	lower := strings.ToLower(model)
	for _, r := range rules {
		if strings.Contains(lower, r.match) {
			for _, c := range r.caps {
				set.Add(c)
			}
		}
	}
	return set
}

// Supports reports whether a model satisfies all required capabilities.
func Supports(model string, required core.CapabilitySet) bool {
	return Of(model).Satisfies(required)
}

// Required infers the capabilities a request needs from its content, so the
// dispatcher can reject incapable fallback targets. It is conservative: it only
// flags capabilities that are unambiguously required by the request shape.
func Required(req *core.ChatRequest) core.CapabilitySet {
	set := core.NewCapabilitySet()
	if len(req.Tools) > 0 {
		set.Add(core.CapToolCalling)
	}
	if req.Stream {
		set.Add(core.CapStreaming)
	}
	if req.Reasoning != nil && (req.Reasoning.Effort != "" || req.Reasoning.MaxTokens > 0) {
		set.Add(core.CapReasoning)
	}
	if len(req.ResponseFormat) > 0 {
		set.Add(core.CapStructuredOutput)
	}
	for _, m := range req.Messages {
		for _, p := range m.Content {
			switch p.Type {
			case core.PartImage:
				set.Add(core.CapVision)
			case core.PartAudio:
				set.Add(core.CapAudioInput)
			}
		}
	}
	return set
}