package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/andrebq/boombox/cmd/boombox/cassette"
	"github.com/andrebq/boombox/cmd/boombox/programs"
	"github.com/andrebq/boombox/cmd/boombox/serve"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "boombox",
		Usage: "Share data and code with everyone!",
		Commands: []*cli.Command{
			cassette.Cmd(),
			serve.Cmd(),
			programs.Cmd(),
		},
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	err := app.RunContext(ctx, os.Args)
	if err != nil {
		log.Error().Err(err).Msg("Application failed")
	}
}
