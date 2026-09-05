package core

import (
	"encoding/json"
	"maps"

	"github.com/alexschlessinger/pollytool/messages"
)

const contextUsageKey = "soulshack_context_usage"

// ContextUsage describes the final provider request in a completed turn. It is
// stored on that turn's assistant message, so clear and expiry also reset it.
type ContextUsage struct {
	EstimatedTokens  int `json:"estimated_tokens"`
	Budget           int `json:"budget"`
	OmittedExchanges int `json:"omitted_exchanges"`
}

func RecordContextUsage(generated []messages.ChatMessage, usage ContextUsage) {
	for i := len(generated) - 1; i >= 0; i-- {
		if generated[i].Role != messages.MessageRoleAssistant {
			continue
		}
		metadata := maps.Clone(generated[i].Metadata)
		if metadata == nil {
			metadata = make(map[string]any)
		}
		metadata[contextUsageKey] = usage
		generated[i].Metadata = metadata
		return
	}
}

func LastContextUsage(history []messages.ChatMessage) (ContextUsage, bool) {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != messages.MessageRoleAssistant {
			continue
		}
		raw, ok := history[i].Metadata[contextUsageKey]
		if !ok {
			continue
		}
		// SQLite round-trips metadata through JSON; accept both fresh and stored values.
		data, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		var usage ContextUsage
		if err := json.Unmarshal(data, &usage); err == nil {
			return usage, true
		}
	}
	return ContextUsage{}, false
}
