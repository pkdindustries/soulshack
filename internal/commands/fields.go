package commands

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/alexschlessinger/pollytool/llm"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/core"
)

type settings = config.Configuration

// configField defines how to get and set a configuration value
type configField struct {
	setter func(*config.Configuration, string) error
	getter func(*config.Configuration) string
	// apply syncs copies held outside the config; runs under the config lock.
	apply        func(*config.Configuration, core.System)
	subagentOnly bool
}

// configFields maps parameter names to their handlers. Setter errors omit the
// key; /set adds it.
var configFields = map[string]configField{
	"addressed":          boolSetting(func(c *settings) *bool { return &c.Bot.Addressed }),
	"prompt":             stringSetting(func(c *settings) *string { return &c.Bot.Prompt }),
	"model":              stringSetting(func(c *settings) *string { return &c.Model.Model }),
	"maxtokens":          intSetting(func(c *settings) *int { return &c.Model.MaxTokens }, 1),
	"temperature":        floatSetting(func(c *settings) *float32 { return &c.Model.Temperature }, atLeast(0)),
	"top_p":              floatSetting(func(c *settings) *float32 { return &c.Model.TopP }, between(0, 1)),
	"openaiurl":          stringSetting(func(c *settings) *string { return &c.API.OpenAIURL }),
	"ollamaurl":          stringSetting(func(c *settings) *string { return &c.API.OllamaURL }),
	"ollamakey":          secretSetting(func(c *settings) *string { return &c.API.OllamaKey }),
	"openaikey":          secretSetting(func(c *settings) *string { return &c.API.OpenAIKey }),
	"anthropickey":       secretSetting(func(c *settings) *string { return &c.API.AnthropicKey }),
	"geminikey":          secretSetting(func(c *settings) *string { return &c.API.GeminiKey }),
	"deepseekkey":        secretSetting(func(c *settings) *string { return &c.API.DeepSeekKey }),
	"openrouterkey":      secretSetting(func(c *settings) *string { return &c.API.OpenRouterKey }),
	"huggingfacekey":     secretSetting(func(c *settings) *string { return &c.API.HuggingFaceKey }),
	"thinkingeffort":     checkedStringSetting(func(c *settings) *string { return &c.Model.ThinkingEffort }, checkThinkingEffort),
	"showthinkingaction": boolSetting(func(c *settings) *bool { return &c.Bot.ShowThinkingAction }),
	"showtoolactions":    boolSetting(func(c *settings) *bool { return &c.Bot.ShowToolActions }),
	"apitimeout":         durationSetting(func(c *settings) *time.Duration { return &c.API.Timeout }, true),
	"chunkmax":           intSetting(func(c *settings) *int { return &c.Session.ChunkMax }, 0),
	"urlwatcher":         boolSetting(func(c *settings) *bool { return &c.Bot.URLWatcher }),
	"urlwatchersilent":   boolSetting(func(c *settings) *bool { return &c.Bot.URLWatcherSilent }),
	"opwatcher":          boolSetting(func(c *settings) *bool { return &c.Bot.OpWatcher }),

	// Read per spawn; whether the tool exists is fixed at startup.
	"subagentmodel":      subagentSetting(stringSetting(func(c *settings) *string { return &c.Bot.SubagentModel })),
	"subagenttimeout":    subagentSetting(durationSetting(func(c *settings) *time.Duration { return &c.Bot.SubagentTimeout }, true)),
	"subagentmax":        subagentSetting(intSetting(func(c *settings) *int { return &c.Bot.SubagentMax }, 0)),
	"subagentmaxperchat": subagentSetting(intSetting(func(c *settings) *int { return &c.Bot.SubagentMaxPerChat }, 0)),

	// Also held by the conversations.
	"maxcontext": withApply(intSetting(func(c *settings) *int { return &c.Session.MaxContext }, 0),
		func(c *config.Configuration, sys core.System) { sys.GetMemory().SetBudget(c.Session.MaxContext) }),
	"sessionduration": withApply(durationSetting(func(c *settings) *time.Duration { return &c.Session.TTL }, false),
		func(c *config.Configuration, sys core.System) { sys.GetMemory().SetTTL(c.Session.TTL) }),
}

var (
	errInteger  = errors.New("provide a valid integer")
	errBoolean  = errors.New("provide 'true' or 'false'")
	errFloat    = errors.New("provide a valid float")
	errDuration = errors.New("provide a valid duration, such as 10m or 1h")
)

func valueSetting[T any](get func(*settings) *T, parse func(string) (T, error), show func(T) string) configField {
	return configField{
		setter: func(c *config.Configuration, v string) error {
			parsed, err := parse(v)
			if err != nil {
				return err
			}
			*get(c) = parsed
			return nil
		},
		getter: func(c *config.Configuration) string { return show(*get(c)) },
	}
}

func boolSetting(get func(*settings) *bool) configField {
	return valueSetting(get, func(v string) (bool, error) {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return false, errBoolean
		}
		return parsed, nil
	}, strconv.FormatBool)
}

// intSetting rejects values below atLeast, if given.
func intSetting(get func(*settings) *int, atLeast ...int) configField {
	minimum, bounded := 0, len(atLeast) > 0
	if bounded {
		minimum = atLeast[0]
	}
	return valueSetting(get, func(v string) (int, error) {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, errInteger
		}
		if bounded && parsed < minimum {
			return 0, fmt.Errorf("provide an integer of at least %d", minimum)
		}
		return parsed, nil
	}, strconv.Itoa)
}

func floatSetting(get func(*settings) *float32, check func(float64) error) configField {
	return valueSetting(get, func(v string) (float32, error) {
		parsed, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return 0, errFloat
		}
		if check != nil {
			if err := check(parsed); err != nil {
				return 0, err
			}
		}
		return float32(parsed), nil
	}, func(f float32) string { return strconv.FormatFloat(float64(f), 'f', 6, 32) })
}

func atLeast(min float64) func(float64) error {
	return func(value float64) error {
		if value < min {
			return fmt.Errorf("provide a float of at least %g", min)
		}
		return nil
	}
}

func between(min, max float64) func(float64) error {
	return func(value float64) error {
		if value < min || value > max {
			return fmt.Errorf("provide a float between %g and %g", min, max)
		}
		return nil
	}
}

func durationSetting(get func(*settings) *time.Duration, positive bool) configField {
	return valueSetting(get, func(v string) (time.Duration, error) {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return 0, errDuration
		}
		if positive && parsed <= 0 {
			return 0, errors.New("provide a positive duration, such as 10m or 1h")
		}
		if !positive && parsed < 0 {
			return 0, errors.New("provide a non-negative duration, such as 10m or 0")
		}
		return parsed, nil
	}, time.Duration.String)
}

func stringSetting(get func(*settings) *string) configField {
	return valueSetting(get, func(v string) (string, error) { return v, nil }, func(s string) string { return s })
}

// secretSetting masks the value on read.
func secretSetting(get func(*settings) *string) configField {
	field := stringSetting(get)
	field.getter = func(c *config.Configuration) string { return maskAPIKey(*get(c)) }
	return field
}

func checkedStringSetting(get func(*settings) *string, check func(string) error) configField {
	field := stringSetting(get)
	field.setter = func(c *config.Configuration, v string) error {
		if err := check(v); err != nil {
			return err
		}
		*get(c) = v
		return nil
	}
	return field
}

func checkThinkingEffort(v string) error {
	_, err := llm.ParseThinkingEffort(v)
	return err
}

func withApply(field configField, apply func(*config.Configuration, core.System)) configField {
	field.apply = apply
	return field
}

func subagentSetting(field configField) configField {
	field.subagentOnly = true
	return field
}

func subagentsEnabled(sys core.System) bool {
	return sys != nil && sys.GetAgents() != nil
}

func lookupConfigField(sys core.System, name string) (configField, bool) {
	field, ok := configFields[name]
	if !ok || (field.subagentOnly && !subagentsEnabled(sys)) {
		return configField{}, false
	}
	return field, true
}

func configKeysFor(sys core.System) []string {
	enabled := subagentsEnabled(sys)
	keys := make([]string, 0, len(configFields))
	for key, field := range configFields {
		if field.subagentOnly && !enabled {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// maskAPIKey returns a masked version of an API key showing only first 4 chars
func maskAPIKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-4)
}
