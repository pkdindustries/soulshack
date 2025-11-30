package main

//  ____                    _   ____    _                      _
// / ___|    ___    _   _  | | / ___|  | |__     __ _    ___  | | __
// \___ \   / _ \  | | | | | | \___ \  | '_ \   / _` |  / __| | |/ /
//  ___) | | (_) | | |_| | | |  ___) | | | | | | (_| | | (__  |   <
// |____/   \___/   \__,_| |_| |____/  |_| |_|  \__,_|  \___| |_|\_\
//  .  .  .  because  real  people  are  overrated

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v3"
	"go.uber.org/zap"

	"pkdindustries/soulshack/internal/bot"
	"pkdindustries/soulshack/internal/config"
)

const version = "0.91"

func main() {
	fmt.Printf("%s\n", bot.GetBanner(version))

	// Create a context that cancels on SIGINT or SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cmd := &cli.Command{
		Name:    "soulshack",
		Usage:   "because real people are overrated",
		Version: version + " - http://github.com/pkdindustries/soulshack",
		Flags:   config.GetFlags(),
		Action: func(_ context.Context, c *cli.Command) error {
			// Use our cancellable context, not the CLI's context
			return bot.Run(ctx, config.NewConfiguration(c))
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		zap.S().Fatal(err)
	}
}
