package connectors

import (
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// TestDynamicProvidersIsolated verifies that two custom provider instances of
// the same base type stay fully separate: distinct ids, distinct base URLs via
// their connectors, and independent model lists.
func TestDynamicProvidersIsolated(t *testing.T) {
	idA := CustomOpenAIPrefix + "vllm"
	idB := CustomOpenAIPrefix + "acme"
	t.Cleanup(func() {
		UnregisterDynamicProvider(idA)
		UnregisterDynamicProvider(idB)
	})

	RegisterDynamicProvider(DynamicProvider{
		ID: idA, DisplayName: "Local vLLM", Alias: idA, Dialect: core.DialectOpenAI, BaseURL: "http://localhost:8000/v1",
	})
	RegisterDynamicProvider(DynamicProvider{
		ID: idB, DisplayName: "Acme Gateway", Alias: idB, Dialect: core.DialectOpenAI, BaseURL: "https://acme.example.com/v1",
	})

	// Both appear in the catalog and resolve via SpecByID.
	specA, okA := SpecByID(idA)
	specB, okB := SpecByID(idB)
	if !okA || !okB {
		t.Fatalf("expected both dynamic providers in catalog: a=%v b=%v", okA, okB)
	}
	if !specA.Custom || !specB.Custom {
		t.Fatalf("dynamic specs should be marked custom")
	}
	if specA.BaseURL == specB.BaseURL {
		t.Fatalf("instances must keep distinct base URLs")
	}

	// Per-instance custom models stay isolated.
	SetDynamicModels(idA, []ModelSpec{{ID: "llama-3", Name: "Llama 3", Kind: core.ServiceLLM}})
	SetDynamicModels(idB, []ModelSpec{{ID: "gpt-4o", Name: "GPT-4o", Kind: core.ServiceLLM}})

	ma := ModelsForProvider(idA)
	mb := ModelsForProvider(idB)
	if len(ma) != 1 || ma[0].ID != "llama-3" {
		t.Fatalf("instance A should only have its own model, got %+v", ma)
	}
	if len(mb) != 1 || mb[0].ID != "gpt-4o" {
		t.Fatalf("instance B should only have its own model, got %+v", mb)
	}

	// The registry builds a working connector for each dynamic provider id.
	reg := DefaultRegistry()
	cA, err := reg.Get(idA)
	if err != nil || cA.ID() != idA {
		t.Fatalf("registry should resolve dynamic connector A: %v", err)
	}
	cB, err := reg.Get(idB)
	if err != nil || cB.ID() != idB {
		t.Fatalf("registry should resolve dynamic connector B: %v", err)
	}
}

// TestModelsForProviderMergesCustom verifies user-defined models merge with the
// built-in catalog and override static entries sharing the same id.
func TestModelsForProviderMergesCustom(t *testing.T) {
	const provider = "openai"
	t.Cleanup(func() { SetDynamicModels(provider, nil) })

	staticCount := len(ModelsForProvider(provider))
	if staticCount == 0 {
		t.Fatal("openai should have static models")
	}

	SetDynamicModels(provider, []ModelSpec{
		{ID: "my-custom", Name: "My Custom", Kind: core.ServiceLLM},
	})
	merged := ModelsForProvider(provider)
	if len(merged) != staticCount+1 {
		t.Fatalf("expected static+1 models, got %d (static %d)", len(merged), staticCount)
	}

	found := false
	for _, m := range merged {
		if m.ID == "my-custom" {
			found = true
		}
	}
	if !found {
		t.Fatal("custom model should appear in merged list")
	}
}
