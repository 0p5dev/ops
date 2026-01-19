package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/0p5dev/ops/internal/auth"
	"github.com/0p5dev/ops/internal/config"
	"github.com/0p5dev/ops/internal/deployment"
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
				Name:    "deployment",
				Aliases: []string{"deploy", "d"},
				Usage:   "Manage deployments",
				Metadata: map[string]any{
					"config": config,
				},
				Commands: []*cli.Command{
					{
						Name:    "create",
						Aliases: []string{"c"},
						Usage:   "Create a deployment",
						Action:  deployment.Create,
						Metadata: map[string]any{
							"config": config,
						},
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:        "min-instances",
								Aliases:     []string{"min"},
								Usage:       "Override minimum number of instances",
								DefaultText: "0",
							},
							&cli.IntFlag{
								Name:        "max-instances",
								Aliases:     []string{"max"},
								Usage:       "Override maximum number of instances",
								DefaultText: "1",
							},
							&cli.IntFlag{
								Name:        "port",
								Aliases:     []string{"p"},
								Usage:       "Override port the application listens on",
								DefaultText: "8080",
							},
							&cli.StringFlag{
								Name:    "file",
								Aliases: []string{"f"},
								Usage:   "Path to Dockerfile or Containerfile",
							},
							&cli.StringFlag{
								Name:    "context",
								Aliases: []string{"c"},
								Usage:   "Docker build context path",
								Value:   ".",
							},
						},
					},
					{
						Name:    "list",
						Aliases: []string{"l"},
						Usage:   "List all deployments",
						Action:  deployment.List,
						Metadata: map[string]any{
							"config": config,
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "output",
								Aliases: []string{"o"},
								Usage:   "Output format: table, json, or yaml",
								Value:   "table",
							},
						},
					},
					{
						Name:   "destroy",
						Usage:  "Destroy a deployment",
						Action: deployment.Destroy,
						Metadata: map[string]any{
							"config": config,
						},
					},
					{
						Name:    "scale",
						Aliases: []string{"s"},
						Usage:   "Scale a deployment's minimum and maximum instances",
						Action:  deployment.Scale,
						Metadata: map[string]any{
							"config": config,
						},
						Flags: []cli.Flag{
							&cli.Int32Flag{
								Name:        "min-instances",
								Aliases:     []string{"min"},
								Usage:       "Minimum number of instances",
								DefaultText: "prompt if not provided",
							},
							&cli.Int32Flag{
								Name:        "max-instances",
								Aliases:     []string{"max"},
								Usage:       "Maximum number of instances",
								DefaultText: "prompt if not provided",
							},
						},
					},
					{
						Name:    "describe",
						Aliases: []string{"d", "get"},
						Usage:   "Get details of a deployment",
						Action:  deployment.Describe,
						Metadata: map[string]any{
							"config": config,
						},
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "output",
								Aliases: []string{"o"},
								Usage:   "Output format: table, json, or yaml",
								Value:   "table",
							},
						},
					},
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
