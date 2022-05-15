package api

import (
	"github.com/andrebq/boombox/cmd/boombox/serve/api/private"
	"github.com/andrebq/boombox/cmd/boombox/serve/api/public"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	return &cli.Command{
		Name:  "api",
		Usage: "Commands to expose a tapedeck as an api (either private or public)",
		Subcommands: []*cli.Command{
			public.Cmd(),
			private.Cmd(),
		},
	}
}
