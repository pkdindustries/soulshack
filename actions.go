package main

import (
	"fmt"
	"log"
	"strings"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
	"github.com/subosito/shorturl"
	gowiki "github.com/trietmn/go-wiki"
)

var actionRegistry = map[string]ReactAction{}

func init() {
	actionRegistry["image"] = &ImageAction{}
	actionRegistry["config"] = &ConfigAction{}
	actionRegistry["wikipedia"] = &WikipediaAction{}
}

type WikipediaAction struct{}

func (a *WikipediaAction) Name() string {
	return "wikipedia"
}

func (a *WikipediaAction) Purpose() string {
	return "if necessary to augment an answer, find current information about people and events from wikipedia"
}

func (a *WikipediaAction) Execute(ctx ChatContext, arg string) (string, error) {

	args := strings.Split(arg, " ")
	if len(args) < 2 {
		return "", fmt.Errorf("wikipedia requires an argument")
	}

	sum, err := gowiki.Summary(strings.Join(args[1:], " "), 0, 2000, true, true)
	if err != nil {
		return "", err
	}

	return sum, nil
}

func (a *WikipediaAction) Spec() string {
	return "wikipedia $topic"
}

type ConfigAction struct{}

func (a *ConfigAction) Name() string {
	return "config"
}

func (a *ConfigAction) Purpose() string {
	return "get or set a bot configuration variable"
}

func (a *ConfigAction) Execute(ctx ChatContext, arg string) (string, error) {

	args := strings.Split(arg, " ")
	_, cmd, k := args[0], args[1], args[2]
	log.Printf("config action: %s %s %s", cmd, k, args)

	// XXX
	if cmd == "set" {
		value := strings.Join(args[3:], " ")
		// set on global config
		vip.Set(k, value)
		if k == "nick" {
			if err := ctx.ChangeName(value); err != nil {
				return "", err
			} else {
				ctx.GetSession().Reset()
			}
		}
		return fmt.Sprintf("%s set to: %s", k, vip.GetString(k)), nil
	} else if cmd == "get" {
		return fmt.Sprintf("%s is: %s", k, vip.GetString(k)), nil
	} else {
		return "", fmt.Errorf("unknown config command: %s, must be get or set", cmd)
	}
}

func (a *ConfigAction) Spec() string {
	return "config get|set $name $value, name one of [temperature, nick, prompt, model, greeting]"
}

type ImageAction struct{}

func (a *ImageAction) Name() string {
	return "image"
}
func (a *ImageAction) Purpose() string {
	return "generates a fictional image based on a description"
}

func (a *ImageAction) Execute(ctx ChatContext, arg string) (string, error) {
	validrez := map[string]bool{
		"256x256":   true,
		"512x512":   true,
		"1024x1024": true,
	}

	args := strings.Split(arg, " ")

	if len(args) < 2 {
		return "", fmt.Errorf("image requires a description")
	}

	prompt := arg
	resolution := "256x256"
	if validrez[args[0]] {
		resolution = args[0]
		prompt = strings.Join(args[1:], " ")
	}

	// ctx.Sendmessage(fmt.Sprintf("creating %s image...", resolution))
	req := ai.ImageRequest{
		Prompt:         prompt,
		Size:           resolution,
		ResponseFormat: ai.CreateImageResponseFormatURL,
		N:              1,
	}

	resp, err := ctx.GetAI().CreateImage(ctx, req)
	if err != nil {
		return "", err
	}
	u, err := shorturl.Shorten(resp.Data[0].URL, "tinyurl")
	if err != nil {
		log.Printf("error shortening url: %v", err)
		return resp.Data[0].URL, nil
	} else {
		return string(u), nil
	}
}

func (a *ImageAction) Spec() string {
	return "image $resolution $description, $resolution one of [256x256, 512x512, 1024x1024], $resolution optional"
}
