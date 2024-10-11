package main

import (
	"encoding/json"

	ai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type IrcOpTool struct {
}

func RegisterIrcTools(registry *ToolRegistry) {
	registry.RegisterTool("irc_op", &IrcOpTool{})
	registry.RegisterTool("irc_kick", &IrcKickTool{})
	registry.RegisterTool("irc_topic", &IrcTopicTool{})
}

func (t *IrcOpTool) GetTool() (ai.Tool, error) {
	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        "irc_op",
			Description: "grants irc operator privileges to a user",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"nick": {
						Type:        jsonschema.String,
						Description: "the irc nickname of the user to grant operator privileges",
					},
					"request_nick": {
						Type:        jsonschema.String,
						Description: "the irc nickname requesting the operator privileges",
					},
					"op": {
						Type:        jsonschema.Boolean,
						Description: "whether to grant or revoke operator privileges",
					},
				},
				Required: []string{"nick", "request_user", "op"},
			},
		}}, nil
}

func (t *IrcOpTool) Execute(ctx ChatContext, tool ai.ToolCall) (ai.ChatCompletionMessage, error) {
	var args json.RawMessage
	var nick, requestNick string
	var op bool
	err := json.Unmarshal(args, &struct {
		Nick        string `json:"nick"`
		RequestNick string `json:"request_nick"`
		Op          bool   `json:"op"`
	}{nick, requestNick, op})
	if err != nil {
		return ai.ChatCompletionMessage{}, err
	}

	// set opcmd to the appropriate value
	opcmd := "-o"
	opmsg := "revoked operator privileges from " + nick
	if op {
		opcmd = "+o"
		opmsg = "granted operator privileges to " + nick
	}

	ctx.Client.Cmd.Mode(nick, opcmd)
	return ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleTool,
		Content: opmsg,
	}, nil
}

type IrcKickTool struct {
}

func (t *IrcKickTool) GetTool() (ai.Tool, error) {
	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        "irc_kick",
			Description: "kicks a user from the channel",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"nick": {
						Type:        jsonschema.String,
						Description: "the irc nickname of the user to kick",
					},
					"reason": {
						Type:        jsonschema.String,
						Description: "the reason for the kick",
					},
					"request_nick": {
						Type:        jsonschema.String,
						Description: "the irc nickname requesting the kick",
					},
				},
				Required: []string{"nick", "reason"},
			},
		}}, nil
}

func (t *IrcKickTool) Execute(ctx ChatContext, tool ai.ToolCall) (ai.ChatCompletionMessage, error) {
	var args json.RawMessage
	var nick, reason, requestNick string
	err := json.Unmarshal(args, &struct {
		Nick        string `json:"nick"`
		Reason      string `json:"reason"`
		RequestNick string `json:"request_nick"`
	}{nick, reason, requestNick})
	if err != nil {
		return ai.ChatCompletionMessage{}, err
	}

	ctx.Client.Cmd.Kick(BotConfig.Channel, nick, reason)
	return ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleTool,
		Content: "kicked " + nick + " (" + reason + ")",
	}, nil
}

type IrcTopicTool struct {
}

func (t *IrcTopicTool) GetTool() (ai.Tool, error) {
	return ai.Tool{
		Type: ai.ToolTypeFunction,
		Function: &ai.FunctionDefinition{
			Name:        "irc_topic",
			Description: "sets the channel topic",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"topic": {
						Type:        jsonschema.String,
						Description: "the new channel topic",
					},
					"request_nick": {
						Type:        jsonschema.String,
						Description: "the irc nickname requesting the topic change",
					},
				},
				Required: []string{"topic"},
			},
		}}, nil
}

func (t *IrcTopicTool) Execute(ctx ChatContext, tool ai.ToolCall) (ai.ChatCompletionMessage, error) {
	var args json.RawMessage
	var topic, requestNick string
	err := json.Unmarshal(args, &struct {
		Topic       string `json:"topic"`
		RequestNick string `json:"request_nick"`
	}{topic, requestNick})
	if err != nil {
		return ai.ChatCompletionMessage{}, err
	}

	ctx.Client.Cmd.Topic(BotConfig.Channel, topic)
	return ai.ChatCompletionMessage{
		Role:    ai.ChatMessageRoleTool,
		Content: requestNick + "set topic to " + topic,
	}, nil
}
