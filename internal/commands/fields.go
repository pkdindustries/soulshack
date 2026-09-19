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

// settings shortens the accessors below: it is the configuration they read.
type settings = config.Configuration

// configField defines how to get and set a configuration value
type configField struct {
	setter func(*config.Configuration, string) error
	getter func(*config.Configuration) string
	// apply pushes a setting to whatever reads it outside the configuration,
	// such as the conversations that hold their own budget and idle expiry.
	// It runs in the same step as the setter, under the configuration lock.
	apply func(*config.Configuration, core.System)
}

// configFields maps parameter names to their handlers. A setter reports why a
// value was rejected without naming the setting: /set adds the name.
var configFields = map[string]configField{
	"addressed":          boolSetting(func(c *settings) *bool { return &c.Bot.Addressed }),
	"prompt":             stringSetting(func(c *settings) *string { return &c.Bot.Prompt }),
	"model":              stringSetting(func(c *settings) *string { return &c.Model.Model }),
	"maxtokens":          intSetting(func(c *settings) *int { return &c.Model.MaxTokens }),
	"temperature":        floatSetting(func(c *settings) *float32 { return &c.Model.Temperature }, nil),
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
	"apitimeout":         durationSetting(func(c *settings) *time.Duration { return &c.API.Timeout }),
	"chunkmax":           intSetting(func(c *settings) *int { return &c.Session.ChunkMax }),
	"urlwatcher":         boolSetting(func(c *settings) *bool { return &c.Bot.URLWatcher }),
	"urlwatchersilent":   boolSetting(func(c *settings) *bool { return &c.Bot.URLWatcherSilent }),
	"opwatcher":          boolSetting(func(c *settings) *bool { return &c.Bot.OpWatcher }),

	// Subagents. Whether the tool exists at all is fixed at startup, since
	// that is when it is registered; everything about how children run is
	// read fresh each time one is spawned.
	"subagentmodel":      stringSetting(func(c *settings) *string { return &c.Bot.SubagentModel }),
	"subagenttimeout":    durationSetting(func(c *settings) *time.Duration { return &c.Bot.SubagentTimeout }),
	"subagentmax":        intSetting(func(c *settings) *int { return &c.Bot.SubagentMax }, 0),
	"subagentmaxperchat": intSetting(func(c *settings) *int { return &c.Bot.SubagentMaxPerChat }, 0),

	// Two settings are also held by the conversations themselves.
	"maxcontext": withApply(intSetting(func(c *settings) *int { return &c.Session.MaxContext }, 0),
		func(c *config.Configuration, sys core.System) { sys.GetMemory().SetBudget(c.Session.MaxContext) }),
	"sessionduration": withApply(durationSetting(func(c *settings) *time.Duration { return &c.Session.TTL }),
		func(c *config.Configuration, sys core.System) { sys.GetMemory().SetTTL(c.Session.TTL) }),
}

var (
	errInteger  = errors.New("provide a valid integer")
	errBoolean  = errors.New("provide 'true' or 'false'")
	errFloat    = errors.New("provide a valid float")
	errDuration = errors.New("provide a valid duration, such as 10m or 1h")
)

// valueSetting builds a field around a parser: parse reports why a value was
// rejected, show renders the stored value back.
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

// intSetting stores an integer, rejecting values below atLeast when it is given.
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

// between rejects floats outside min and max.
func between(min, max float64) func(float64) error {
	return func(value float64) error {
		if value < min || value > max {
			return fmt.Errorf("provide a float between %g and %g", min, max)
		}
		return nil
	}
}

func durationSetting(get func(*settings) *time.Duration) configField {
	return valueSetting(get, func(v string) (time.Duration, error) {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return 0, errDuration
		}
		return parsed, nil
	}, time.Duration.String)
}

func stringSetting(get func(*settings) *string) configField {
	return valueSetting(get, func(v string) (string, error) { return v, nil }, func(s string) string { return s })
}

// secretSetting stores a value that is masked when it is read back.
func secretSetting(get func(*settings) *string) configField {
	field := stringSetting(get)
	field.getter = func(c *config.Configuration) string { return maskAPIKey(*get(c)) }
	return field
}

// checkedStringSetting stores a string once check accepts it.
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

// withApply attaches the step that pushes a setting to whatever reads it
// outside the configuration.
func withApply(field configField, apply func(*config.Configuration, core.System)) configField {
	field.apply = apply
	return field
}

// getConfigKeys returns all available config keys, in a stable order
func getConfigKeys() []string {
	keys := make([]string, 0, len(configFields))
	for key := range configFields {
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
