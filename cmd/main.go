package main

import (
	"context"
	"fmt"
	"os"

	"github.com/0p5dev/ops/internal/auth"
	"github.com/0p5dev/ops/internal/config"
	"github.com/0p5dev/ops/internal/deployment"
	"github.com/urfave/cli/v3"
)

var Version string

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	cli.VersionPrinter = func(cmd *cli.Command) {
		fmt.Printf("ops %s\n", Version)
	}

	cmd := &cli.Command{
		Version:                Version,
		Name:                   "ops",
		Usage:                  "A CLI tool to deploy developer-first, autoscaling applications",
		UseShortOptionHandling: true,
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
							&cli.BoolFlag{
								Name:    "yes",
								Aliases: []string{"y"},
								Usage:   "Automatically confirm prompts",
							},
							&cli.BoolFlag{
								Name:    "no-wait",
								Aliases: []string{"n"},
								Usage:   "Do not wait for the deployment to complete",
							},
						},
					},
					{
						Name:    "update",
						Aliases: []string{"u"},
						Usage:   "Update a deployment",
						Action:  deployment.Update,
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
							&cli.BoolFlag{
								Name:    "yes",
								Aliases: []string{"y"},
								Usage:   "Automatically confirm prompts",
							},
							&cli.BoolFlag{
								Name:    "no-wait",
								Aliases: []string{"n"},
								Usage:   "Do not wait for the deployment to complete",
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
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:    "yes",
								Aliases: []string{"y"},
								Usage:   "Automatically confirm prompts",
							},
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
							&cli.BoolFlag{
								Name:    "yes",
								Aliases: []string{"y"},
								Usage:   "Automatically confirm prompts",
							},
							&cli.BoolFlag{
								Name:    "no-wait",
								Aliases: []string{"n"},
								Usage:   "Do not wait for the scale operation to complete",
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
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "no-open",
						Aliases: []string{"n"},
						Usage:   "Do not open the browser automatically",
					},
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Printf("command failed: %v\n", err)
		os.Exit(1)
	}
}
