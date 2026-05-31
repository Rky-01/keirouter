package terse

import (
	"strings"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/stretchr/testify/require"
)

func TestApply_Disabled(t *testing.T) {
	req := &core.ChatRequest{System: "original instructions"}
	Apply(req, Config{Enabled: false, Level: LevelAggressive})
	require.Equal(t, "original instructions", req.System)
}

func TestApply_NilRequest(t *testing.T) {
	require.NotPanics(t, func() {
		Apply(nil, Config{Enabled: true, Level: LevelMedium})
	})
}

func TestApply_InjectsPerLevel(t *testing.T) {
	tests := []struct {
		name string
		lvl  Level
		want string
	}{
		{"light", LevelLight, instructionLight},
		{"medium", LevelMedium, instructionMedium},
		{"aggressive", LevelAggressive, instructionAggressive},
		{"unknown defaults to medium", Level("bogus"), instructionMedium},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &core.ChatRequest{}
			Apply(req, Config{Enabled: true, Level: tc.lvl})
			require.Contains(t, req.System, sentinel)
			require.Contains(t, req.System, tc.want)
		})
	}
}

func TestApply_PrependsToExistingSystem(t *testing.T) {
	req := &core.ChatRequest{System: "you are a helpful assistant"}
	Apply(req, Config{Enabled: true, Level: LevelMedium})

	require.Contains(t, req.System, sentinel)
	require.Contains(t, req.System, instructionMedium)
	require.Contains(t, req.System, "you are a helpful assistant")
	// terse block must come first
	require.True(t, strings.Index(req.System, sentinel) < strings.Index(req.System, "you are a helpful assistant"))
}

func TestApply_Idempotent(t *testing.T) {
	req := &core.ChatRequest{System: "base"}
	Apply(req, Config{Enabled: true, Level: LevelAggressive})
	first := req.System
	Apply(req, Config{Enabled: true, Level: LevelAggressive})
	require.Equal(t, first, req.System, "second apply must not change anything")
	require.Equal(t, 1, strings.Count(req.System, sentinel), "sentinel must appear exactly once")
}