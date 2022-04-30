package serve

import "github.com/urfave/cli/v2"

func Cmd() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start a boombox instance",
		Action: func(ctx *cli.Context) error {
			<-ctx.Context.Done()
			return ctx.Context.Err()
		},
	}
}
