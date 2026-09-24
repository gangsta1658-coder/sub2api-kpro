package service

import (
	"sort"
	"strings"
)

// TestPickerModel is the compact catalog used by the account test dialog.
type TestPickerModel struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
}

// ConfiguredTestModelIDs returns concrete model IDs from an account's explicit
// model_mapping. It intentionally reads the raw configured keys instead of
// GetModelMapping, which may synthesize platform defaults or aliases. An empty
// result means no explicit restriction, so callers may use the platform's
// default or discovered catalog. Wildcard keys are not selectable targets.
func ConfiguredTestModelIDs(account *Account) []string {
	if account == nil || account.IsOpenAIPassthroughEnabled() {
		return nil
	}

	var configuredIDs []string
	switch mapping := account.Credentials["model_mapping"].(type) {
	case map[string]any:
		if len(mapping) == 0 {
			return nil
		}
		for id := range mapping {
			configuredIDs = append(configuredIDs, id)
		}
	case map[string]string:
		if len(mapping) == 0 {
			return nil
		}
		for id := range mapping {
			configuredIDs = append(configuredIDs, id)
		}
	default:
		return nil
	}

	ids := make([]string, 0, len(configuredIDs))
	seen := make(map[string]struct{}, len(configuredIDs))
	for _, id := range configuredIDs {
		id = strings.TrimSpace(id)
		if id == "" || strings.Contains(id, "*") {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// TestPickerModels converts configured model IDs into the common picker DTO.
func TestPickerModels(ids []string) []TestPickerModel {
	out := make([]TestPickerModel, 0, len(ids))
	for _, id := range ids {
		out = append(out, TestPickerModel{ID: id, Type: "model", DisplayName: id})
	}
	return out
}
