package main

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type MockAction struct {
	name    string
	purpose string
	spec    string
	example string
}

func (m *MockAction) Name() string {
	return m.name
}

func (m *MockAction) Purpose() string {
	return m.purpose
}

func (a *MockAction) Execute(c ChatContext, input string) (string, error) {
	if input == "error" {
		return "", errors.New("an error occurred")
	}
	return "output", nil
}

func (a *MockAction) Spec() string {
	return a.spec
}

func (a *MockAction) Example() string {
	return a.example
}

func TestReactPrompt(t *testing.T) {
	config := &ReactConfig{
		Actions: map[string]ReactAction{
			"config": &ConfigAction{},
			"image":  &ImageAction{},
		},
	}

	expected := `you have access to the following tools:
config:get or set a bot configuration variable
example: config get|set $name $value, name one of [temperature, nick, prompt, model, greeting]
image:generates a fictional image based on a description
example: image $resolution $description, $resolution one of [256x256, 512x512, 1024x1024], $resolution optional
Use the following format:
Question: the input question you must answer
Thought: you should always think about what to do
Action: the action to take, should be one of [{$toolname}]

** you will issue a single Action at a time, and you will then STOP and wait for an Observation reply **

Observation: the result of the action
... (this Thought/Action/Observation can repeat N times)
Thought: I now know the final answer
Final Answer: the final answer to the original input question

Question:`

	result := ReactPrompt(config)
	if diff := cmp.Diff(expected, result); diff != "" {
		t.Errorf("Strings do not match (-expected +actual):\n%s", diff)
	}
}

func TestReactor(t *testing.T) {
	mockConf := &ReactConfig{
		Actions: map[string]ReactAction{
			"mock": &MockAction{
				name:    "mock",
				purpose: "mock purpose",
			},
		},
	}
	in := make(chan string)
	out := ReactFilter(&DiscordContext{}, mockConf, in)
	in <- "Action: mock input"
	close(in)

	for s := range out {
		if diff := cmp.Diff(s, "Observation: output"); diff != "" {
			t.Errorf("Strings do not match (-expected +actual):\n%s", diff)
		}

	}

}
