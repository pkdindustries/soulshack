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

var RootCmd = &cobra.Command{
	Use:     "soulshack",
	Example: "soulshack irc|discord",
	Short:   GetBanner(),
	Version: "0.50 - http://github.com/pkdindustries/soulshack",
}

var IrcCmd = &cobra.Command{
	Use:     "irc",
	Example: "soulshack irc",
	Short:   "run soulshack as an irc bot",
	Run:     runIrc,
}

var DiscordCmd = &cobra.Command{
	Use:     "discord",
	Example: "soulshack discord",
	Short:   "run soulshack as a discord bot",
	Run:     runDiscord,
}

func main() {
	RootCmd.AddCommand(IrcCmd)
	RootCmd.AddCommand(DiscordCmd)
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runIrc(_ *cobra.Command, _ []string) {
	log.Println("running soulshack as an irc bot")
	aiClient := ai.NewClient(vip.GetString("openaikey"))

	if err := VerifyConfig(vip.GetViper()); err != nil {
		log.Fatal(err)
	}

	go Irc(aiClient)
	select {}
}

func runDiscord(_ *cobra.Command, _ []string) {
	log.Println("running soulshack as a discord bot")
	aiClient := ai.NewClient(vip.GetString("openaikey"))

	if err := VerifyConfig(vip.GetViper()); err != nil {
		log.Fatal(err)
	}

	go Discord(aiClient)
	select {}
}

func GetBanner() string {
	return fmt.Sprintf("%s\n%s",
		figure.NewColorFigure("SoulShack", "", "green", true).ColorString(),
		figure.NewColorFigure(" . . . because real people are overrated", "term", "green", true).ColorString())
}
