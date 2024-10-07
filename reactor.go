package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

var regex = regexp.MustCompile(`(^|\n)Action:`)

type ReactAction interface {
	Name() string
	Purpose() string
	Execute(*ChatContext, string) (string, error)
	Spec() string
}

type ReactConfig struct {
	Actions map[string]ReactAction
}

func ReactPrompt(config *ReactConfig) string {
	var tools []string
	for _, action := range config.Actions {
		tools = append(tools, fmt.Sprintf("%s:%s", action.Name(), action.Purpose()))
		tools = append(tools, fmt.Sprintf("example: %s", action.Spec()))
	}

	template := fmt.Sprintf(`you have access to the following tools:
%s
Use the following format:
Question: the input question you must answer
Thought: you should always think about what to do
Action: the action to take, should be one of [{$toolname}]

** you will issue a single Action at a time, and you will then STOP and wait for an Observation reply **

Observation: the result of the action
... (this Thought/Action/Observation can repeat N times)
Thought: I now know the final answer
Final Answer: the final answer to the original input question

Question:`, strings.Join(tools, "\n"))

	return template
}

func ReactObservation(ctx *ChatContext, msg string) {
	if action := ReactFindActions(msg); action != "" {
		log.Println("action found:", action)
		r, e := React(ctx, &ReactConfig{Actions: map[string]ReactAction{}}, strings.TrimSpace(action))
		if e != nil {
			log.Println(e)
			Complete(ctx, RoleAssistant, "Observation: "+e.Error())
		} else {
			log.Println(r)
			Complete(ctx, RoleAssistant, "Observation: "+r)
		}
	}
}

func ReactFindActions(msg string) string {
	if loc := regex.FindStringIndex(msg); loc != nil {
		action := msg[loc[0]:]
		return action
	}
	return ""
}

func React(ctx *ChatContext, cfg *ReactConfig, msg string) (string, error) {
	parts := strings.SplitN(msg, ": ", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid input: %s", msg)
	}
	args := strings.Split(parts[1], " ")
	action := strings.TrimSpace(args[0])

	ra, ok := cfg.Actions[action]
	if !ok {
		log.Println("Action not found:", action)
		return "", fmt.Errorf("action not found: %s", action)
	}

	return ra.Execute(ctx, parts[1])
}
