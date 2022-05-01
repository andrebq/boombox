package serve

import (
	"path/filepath"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/internal/httpserver"
	"github.com/andrebq/boombox/tapedeck"
	"github.com/andrebq/boombox/tapedeck/api"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	bindAddr := "localhost:7007"
	var tapes cli.StringSlice
	idxCassette := "index"
	return &cli.Command{
		Name:  "serve",
		Usage: "Start a boombox instance",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "bind",
				Usage:       "Address to bind and export the cassettes",
				Value:       bindAddr,
				Destination: &bindAddr,
			},
			&cli.StringSliceFlag{
				Name:        "cassette",
				Aliases:     []string{"t", "tape"},
				Usage:       "Path to a control cassette (basename will be used as tape name)",
				Destination: &tapes,
			},
			&cli.StringFlag{
				Name:        "index",
				Aliases:     []string{"i"},
				Usage:       "Cassette name (checke cassette flag) to use as the index cassette (ie.: accessible with cassette prefix path)",
				Value:       idxCassette,
				Destination: &idxCassette,
			},
		},
		Action: func(ctx *cli.Context) error {
			deck := tapedeck.New()
			defer deck.Close()
			for _, t := range tapes.Value() {
				c, err := cassette.LoadControlCassette(ctx.Context, t, false)
				if err != nil {
					return err
				}
				tapeName := filepath.Base(t)
				deck.Load(tapeName[:len(tapeName)-len(filepath.Ext(tapeName))], c)
			}
			deck.IndexCassette(idxCassette)
			handler, err := api.AsHandler(ctx.Context, deck)
			if err != nil {
				return err
			}
			return httpserver.Serve(ctx.Context, bindAddr, handler)
		},
	}
}
