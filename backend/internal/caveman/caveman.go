// Package caveman implements output-token compression by injecting a terse
// "caveman speak" instruction into the request's system prompt.
//
// It is a faithful Go port of 9router's caveman injector, which itself adapts
// the caveman skill (https://github.com/JuliusBrussee/caveman). The model is
// instructed to keep all technical substance exact while dropping articles,
// filler, hedging, and pleasantries — cutting roughly 65-75% of output tokens
// without losing meaning.
//
// Like terse mode, this is a request-side transform that only touches the
// system prompt and runs before format translation, so it applies uniformly
// across every provider dialect.
package caveman

import (
	"strings"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// sentinel marks the injected instruction so Apply is idempotent across retries
// and fallbacks. Models ignore HTML-style comments in their output.
const sentinel = "<!-- keirouter:caveman -->"

// Level selects how aggressively the model compresses its output.
type Level string

const (
	// LevelLite keeps grammar and full sentences but drops filler and hedging.
	LevelLite Level = "lite"
	// LevelFull uses terse caveman style: fragments, dropped articles, short
	// synonyms, while keeping all technical substance exact.
	LevelFull Level = "full"
	// LevelUltra is maximally compressed and telegraphic, with abbreviations
	// and causal arrows.
	LevelUltra Level = "ultra"
)

// Config controls caveman behavior for a request.
type Config struct {
	Enabled bool
	Level   Level
}

// sharedExamples shows the contrast between filler-heavy and terse replies.
const sharedExamples = "Not: \"Sure! I'd be happy to help you with that. " +
	"The issue you're experiencing is likely caused by...\" " +
	"Yes: \"Bug in auth middleware. Token expiry check use `<` not `<=`. Fix:\""

// sharedBoundaries protects content that must never be compressed.
const sharedBoundaries = "Code blocks, file paths, commands, errors, URLs: keep exact. " +
	"Security warnings, irreversible action confirmations, multi-step ordered sequences: write normal. " +
	"Resume terse style after."

// sharedAutoClarity tells the model when to temporarily drop terse style.
const sharedAutoClarity = "Auto-Clarity: drop caveman for security warnings, irreversible actions, " +
	"multi-step sequences where fragment ambiguity risks misread, or when user repeats a question. " +
	"Resume after the clear part."

// sharedPersistence keeps the directive active across a long conversation.
const sharedPersistence = "ACTIVE EVERY RESPONSE. No revert after many turns. No filler drift. Still active if unsure."

const promptLite = "Respond tersely. Keep grammar and full sentences but drop filler, hedging and pleasantries " +
	"(just/really/basically/sure/of course/I'd be happy to). " +
	"Pattern: state the thing, the action, the reason. Then next step. " +
	sharedExamples + " " +
	sharedBoundaries + " " +
	sharedAutoClarity + " " +
	sharedPersistence

const promptFull = "Respond like terse caveman. All technical substance stay exact, only fluff die. " +
	"Drop: articles (a/an/the), filler (just/really/basically/actually/simply), pleasantries, hedging. " +
	"Fragments OK. Short synonyms (big not extensive, fix not implement a solution for). " +
	"Pattern: [thing] [action] [reason]. [next step]. " +
	sharedExamples + " " +
	sharedBoundaries + " " +
	sharedAutoClarity + " " +
	sharedPersistence

const promptUltra = "Respond ultra-terse. Maximum compression. Telegraphic. " +
	"Abbreviate (DB/auth/config/req/res/fn/impl), strip conjunctions, use arrows for causality (X → Y). " +
	"One word when one word enough. " +
	"Pattern: [thing] → [result]. [fix]. " +
	sharedExamples + " " +
	sharedBoundaries + " " +
	sharedAutoClarity + " " +
	sharedPersistence

// promptFor returns the instruction text for a level, defaulting to full.
func promptFor(l Level) string {
	switch l {
	case LevelLite:
		return promptLite
	case LevelUltra:
		return promptUltra
	case LevelFull:
		return promptFull
	default:
		return promptFull
	}
}

// Apply injects the caveman instruction into req.System when cfg.Enabled.
//
// It is a no-op when disabled or when already applied (detected via the
// sentinel), so it is safe across retries and fallback attempts. The block is
// appended after any existing system text so the user's own instructions keep
// priority while the caveman directive still takes effect.
func Apply(req *core.ChatRequest, cfg Config) {
	if req == nil || !cfg.Enabled {
		return
	}
	if strings.Contains(req.System, sentinel) {
		return
	}

	block := sentinel + "\n" + promptFor(cfg.Level)
	if strings.TrimSpace(req.System) == "" {
		req.System = block
		return
	}
	req.System = req.System + "\n\n" + block
}

// ValidLevel reports whether s is a recognized caveman level.
func ValidLevel(s Level) bool {
	switch s {
	case LevelLite, LevelFull, LevelUltra:
		return true
	default:
		return false
	}
}
