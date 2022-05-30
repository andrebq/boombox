package programs

import (
	"github.com/andrebq/boombox/cmd/boombox/programs/authprogram"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	return &cli.Command{
		Name:    "programs",
		Aliases: []string{"p"},
		Usage:   "Execute programs (either builtin to boombox or from cassettes)",
		Subcommands: []*cli.Command{
			authprogram.Cmd(),
		},
	}
}
