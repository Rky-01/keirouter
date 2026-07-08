package caveman

import (
	"strings"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
)

func TestApplyDisabled(t *testing.T) {
	req := &core.ChatRequest{System: "original"}
	Apply(req, Config{Enabled: false})
	if req.System != "original" {
		t.Fatalf("disabled caveman must not change system, got %q", req.System)
	}
}

func TestApplyInjectsIntoEmptySystem(t *testing.T) {
	req := &core.ChatRequest{}
	Apply(req, Config{Enabled: true, Level: LevelFull})
	if !strings.Contains(req.System, idempotencyProbe) {
		t.Fatal("expected style directive in system prompt")
	}
	if !strings.Contains(req.System, "terse, information-dense") {
		t.Fatalf("expected full-level prompt, got %q", req.System)
	}
}

func TestApplyAddsNoVisibleMarkerOrSelfName(t *testing.T) {
	req := &core.ChatRequest{}
	Apply(req, Config{Enabled: true, Level: LevelFull})
	for _, banned := range []string{"<!--", "keirouter:", "smart caveman"} {
		if strings.Contains(req.System, banned) {
			t.Errorf("system prompt must not contain %q, got %q", banned, req.System)
		}
	}
}

func TestApplyAppendsToExistingSystem(t *testing.T) {
	req := &core.ChatRequest{System: "be helpful"}
	Apply(req, Config{Enabled: true, Level: LevelLite})
	if !strings.HasPrefix(req.System, "be helpful") {
		t.Fatalf("existing system text must be preserved first, got %q", req.System)
	}
	if !strings.Contains(req.System, idempotencyProbe) {
		t.Fatal("expected style directive appended")
	}
}

func TestApplyIdempotent(t *testing.T) {
	req := &core.ChatRequest{}
	Apply(req, Config{Enabled: true, Level: LevelFull})
	first := req.System
	Apply(req, Config{Enabled: true, Level: LevelFull})
	if req.System != first {
		t.Fatal("second Apply must be a no-op")
	}
}

func TestLevelSelection(t *testing.T) {
	cases := map[Level]string{
		LevelLite:  "Keep grammar",
		LevelFull:  "terse, information-dense",
		LevelUltra: "ultra-terse",
	}
	for level, want := range cases {
		req := &core.ChatRequest{}
		Apply(req, Config{Enabled: true, Level: level})
		if !strings.Contains(req.System, want) {
			t.Errorf("level %q: expected %q, got %q", level, want, req.System)
		}
	}
}

func TestValidLevel(t *testing.T) {
	for _, l := range []Level{LevelLite, LevelFull, LevelUltra,
		LevelWenyanLite, LevelWenyanFull, LevelWenyanUltra} {
		if !ValidLevel(l) {
			t.Errorf("expected %q to be valid", l)
		}
	}
	if ValidLevel("bogus") {
		t.Error("expected bogus level to be invalid")
	}
}

func TestWenyanLevelSelection(t *testing.T) {
	cases := map[Level]string{
		LevelWenyanLite:  "semi-classical",
		LevelWenyanFull:  "文言文",
		LevelWenyanUltra: "classical Chinese feel",
	}
	for level, want := range cases {
		req := &core.ChatRequest{}
		Apply(req, Config{Enabled: true, Level: level})
		if !strings.Contains(req.System, want) {
			t.Errorf("level %q: expected %q, got %q", level, want, req.System)
		}
	}
}

func TestSharedNoInventedAbbrevInAllLevels(t *testing.T) {
	for _, level := range []Level{LevelLite, LevelFull, LevelUltra,
		LevelWenyanLite, LevelWenyanFull, LevelWenyanUltra} {
		p := promptFor(level)
		if !strings.Contains(p, "No invented abbreviations") {
			t.Errorf("level %q: missing sharedNoInventedAbbrev", level)
		}
	}
}

func TestSharedPreserveLanguageInAllLevels(t *testing.T) {
	for _, level := range []Level{LevelLite, LevelFull, LevelUltra,
		LevelWenyanLite, LevelWenyanFull, LevelWenyanUltra} {
		p := promptFor(level)
		if !strings.Contains(p, "Preserve the user's dominant language") {
			t.Errorf("level %q: missing sharedPreserveLanguage", level)
		}
	}
}

func TestSharedNoSelfReferenceInAllLevels(t *testing.T) {
	for _, level := range []Level{LevelLite, LevelFull, LevelUltra,
		LevelWenyanLite, LevelWenyanFull, LevelWenyanUltra} {
		p := promptFor(level)
		if !strings.Contains(p, "No self-reference") {
			t.Errorf("level %q: missing sharedNoSelfReference", level)
		}
	}
}

func TestSharedNoDecorationInAllLevels(t *testing.T) {
	for _, level := range []Level{LevelLite, LevelFull, LevelUltra,
		LevelWenyanLite, LevelWenyanFull, LevelWenyanUltra} {
		p := promptFor(level)
		if !strings.Contains(p, "No decorative emoji") {
			t.Errorf("level %q: missing sharedNoDecoration", level)
		}
	}
}

func TestUltraNoContradictions(t *testing.T) {
	p := promptFor(LevelUltra)
	if strings.Contains(p, "req/res/fn/impl") {
		t.Error("ultra must not contain 'req/res/fn/impl'")
	}
	if strings.Contains(p, "Abbreviate prose words") {
		t.Error("ultra must not contain 'Abbreviate prose words'")
	}
}

func TestSharedBoundariesEnhanced(t *testing.T) {
	for _, want := range []string{
		"Do not describe or announce this style",
		"Preserve user's dominant language",
		"ALWAYS keep technical terms",
		"CLI commands",
	} {
		if !strings.Contains(sharedBoundaries, want) {
			t.Errorf("sharedBoundaries missing %q", want)
		}
	}
}

func TestSharedAutoClarityEnhanced(t *testing.T) {
	if !strings.Contains(sharedAutoClarity, "compression itself creates technical ambiguity") {
		t.Error("sharedAutoClarity missing technical ambiguity condition")
	}
}