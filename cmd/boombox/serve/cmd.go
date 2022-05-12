package serve

import (
	"github.com/andrebq/boombox/cmd/boombox/serve/api"
	"github.com/andrebq/boombox/cmd/boombox/serve/query"
	"github.com/andrebq/boombox/cmd/boombox/serve/router"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Root command to start various boombox services",
		Subcommands: []*cli.Command{
			api.Cmd(),
			router.Cmd(),
			query.Cmd(),
		},
	}
}
