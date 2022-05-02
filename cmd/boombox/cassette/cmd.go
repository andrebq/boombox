package cassette

import (
	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/cassette/importer"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	var tape string
	return &cli.Command{
		Name:    "cassette",
		Aliases: []string{"k7", "tapes"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "file",
				Aliases:     []string{"f"},
				Usage:       "Filepath to the the cassette tape",
				Required:    true,
				Destination: &tape,
			},
		},
		Subcommands: []*cli.Command{
			importCmd(&tape),
		},
	}
}

func importCmd(tape *string) *cli.Command {
	var dir string
	var nocode bool
	return &cli.Command{
		Name:    "import",
		Aliases: []string{"i"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "dir",
				Usage:       "Directory to import as cassette",
				Required:    true,
				Destination: &dir,
			},
			&cli.BoolFlag{
				Name:        "nocode",
				Usage:       "Disable codebase imports",
				Destination: &nocode,
			},
		},
		Action: func(ctx *cli.Context) error {
			k7, err := cassette.LoadControlCassette(ctx.Context, *tape, true, true)
			if err != nil {
				return err
			}
			err = importer.Directory(ctx.Context, k7, dir, !nocode)
			k7.Close()
			return err
		},
	}
}
