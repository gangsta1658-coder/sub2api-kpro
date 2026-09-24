package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfiguredTestModelIDsUsesExplicitMappingKeys(t *testing.T) {
	account := &Account{
		Platform: PlatformGemini,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gemini-3.8-flash": "upstream-flash",
				"claude-sonnet-4.6": "upstream-sonnet",
				"gemini-*":          "gemini-default",
				"  ":                "ignored",
			},
		},
	}

	require.Equal(t, []string{"claude-sonnet-4.6", "gemini-3.8-flash"}, ConfiguredTestModelIDs(account))
}

func TestConfiguredTestModelIDsDoesNotReintroduceSyntheticAntigravityModels(t *testing.T) {
	account := &Account{
		Platform: PlatformAntigravity,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gemini-3.8-flash-high": "gemini-3.8-flash-high"},
		},
	}

	// Runtime mapping adds compatibility defaults, but the test picker must use
	// only the model IDs explicitly configured on the account.
	require.Contains(t, account.GetModelMapping(), "gemini-3.7-flash")
	require.Equal(t, []string{"gemini-3.8-flash-high"}, ConfiguredTestModelIDs(account))
}

func TestConfiguredTestModelIDsEmptyMappingAllowsPlatformFallback(t *testing.T) {
	require.Nil(t, ConfiguredTestModelIDs(&Account{Platform: PlatformAntigravity}))
}

func TestConfiguredTestModelIDsOpenAIPassthroughAllowsUpstreamCatalog(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-test": "gpt-upstream"},
		},
		Extra: map[string]any{"openai_passthrough": true},
	}

	require.Nil(t, ConfiguredTestModelIDs(account))
}

func TestTestPickerModelsBuildsCommonDTO(t *testing.T) {
	require.Equal(t, []TestPickerModel{{ID: "model-a", Type: "model", DisplayName: "model-a"}}, TestPickerModels([]string{"model-a"}))
	require.Empty(t, TestPickerModels(nil))
}
