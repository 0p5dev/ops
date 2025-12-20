package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/0p5dev/ops/internal/auth"
	"github.com/0p5dev/ops/internal/config"
	"github.com/0p5dev/ops/internal/deploy"
	"github.com/urfave/cli/v3"
)

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	cmd := &cli.Command{
		Name:  "ops",
		Usage: "A CLI tool to deploy developer-first, autoscaling applications",
		Commands: []*cli.Command{
			{
				Name:    "deploy",
				Aliases: []string{"d"},
				Usage:   "Deploy a project",
				Action:  deploy.Deploy,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:        "min-instances",
						Usage:       "Override minimum number of instances",
						DefaultText: "0",
					},
					&cli.IntFlag{
						Name:        "max-instances",
						Usage:       "Override maximum number of instances",
						DefaultText: "1",
					},
					&cli.IntFlag{
						Name:        "port",
						Usage:       "Override port the application listens on",
						DefaultText: "8080",
					},
				},
				Metadata: map[string]any{
					"config": config,
				},
			},
			{
				Name:    "login",
				Aliases: []string{"l"},
				Usage:   "Login to 0p5.dev",
				Action:  auth.Login,
				Metadata: map[string]any{
					"config": config,
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
