// Package terse implements KeiRouter's output-token saving mode.
//
// When enabled, Apply injects a system instruction that steers the model
// toward terse responses: technical substance (code, commands, concrete
// answers) is preserved while conversational filler, restated questions, and
// redundant prose are dropped. Compression scales with the configured Level.
//
// This is a request-side transform: it only modifies the system prompt, never
// the user's messages, and runs before format translation so it applies
// uniformly across every provider dialect.
package terse

import (
	"strings"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// sentinel marks instruction text KeiRouter injected, so Apply is idempotent
// across retries and chained fallbacks. It is an HTML-style comment that models
// ignore in their output.
const sentinel = "<!-- keirouter:terse -->"

// Level selects how aggressively responses are compressed.
type Level string

const (
	// LevelLight trims pleasantries and asks for concise answers.
	LevelLight Level = "light"
	// LevelMedium favors bullets and minimal prose while keeping all detail.
	LevelMedium Level = "medium"
	// LevelAggressive compresses to the bare technical minimum.
	LevelAggressive Level = "aggressive"
)

// Config controls terse-mode behavior for a request.
type Config struct {
	Enabled bool
	Level   Level
}

const instructionLight = `Response style: be concise and direct.
- Skip greetings, sign-offs, and filler ("Sure!", "Great question", "I hope this helps").
- Do not restate the question before answering.
- Keep all code, commands, and technical detail intact.`

const instructionMedium = `Response style: terse and information-dense.
- Lead with the answer; omit preamble and summaries of what you are about to do.
- Prefer short bullet points and code blocks over paragraphs.
- Strip filler, hedging, and motivational phrasing.
- Preserve every code snippet, command, file path, and exact value verbatim.
- Explain only when correctness depends on it.`

const instructionAggressive = `Response style: maximum compression.
- Output only what is required to answer. No preamble, no recap, no closing remarks.
- Never restate the question or describe your plan.
- Sentence fragments and note form are acceptable.
- Code, commands, and exact values over prose; show, do not narrate.
- Omit explanations and rationale unless explicitly requested.
- One concrete answer beats three qualified ones.`

// instructionFor returns the prompt text for the given level, defaulting to
// medium for unknown values.
func instructionFor(l Level) string {
	switch l {
	case LevelLight:
		return instructionLight
	case LevelAggressive:
		return instructionAggressive
	case LevelMedium:
		return instructionMedium
	default:
		return instructionMedium
	}
}

// Apply injects the terse-mode instruction into req.System when cfg.Enabled.
//
// It is a no-op when disabled or when the instruction has already been applied
// (detected via the sentinel marker), so it is safe to call repeatedly across
// retries and fallback attempts. The injected block is prepended so the terse
// directive takes precedence over any pre-existing system text.
func Apply(req *core.ChatRequest, cfg Config) {
	if req == nil || !cfg.Enabled {
		return
	}
	if strings.Contains(req.System, sentinel) {
		return
	}

	block := sentinel + "\n" + instructionFor(cfg.Level)
	if strings.TrimSpace(req.System) == "" {
		req.System = block
		return
	}
	req.System = block + "\n\n" + req.System
}