package main

//  ____                    _   ____    _                      _
// / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
// \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
//  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
// |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
//  .  .  .  because  real  people  are  overrated

import (
	"fmt"
	"log"

	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"
	vip "github.com/spf13/viper"

	ai "github.com/sashabaranov/go-openai"
)

func getBanner() string {
	return fmt.Sprintf("%s\n%s",
		figure.NewColorFigure("SoulShack", "", "green", true).ColorString(),
		figure.NewColorFigure(" . . . because real people are overrated", "term", "green", true).ColorString())
}

func main() {
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

var root = &cobra.Command{
	Use:     "soulshack",
	Example: "soulshack --nick chatbot --server irc.freenode.net --port 6697 --channel '#soulshack' --ssl --openaikey ****************",
	Short:   getBanner(),
	Run:     run,
	Version: "0.42 - http://github.com/pkdindustries/soulshack",
}

func run(_ *cobra.Command, _ []string) {

	aiClient := ai.NewClient(vip.GetString("openaikey"))

	if err := verifyConfig(vip.GetViper()); err != nil {
		log.Fatal(err)
	}

	if len(vip.GetString("discordtoken")) > 0 {
		go startDiscord(aiClient)
	} else {
		go startIrc(aiClient)
	}

	select {}
}
