package query

import (
	"path/filepath"

	"github.com/andrebq/boombox/cassette"
	capi "github.com/andrebq/boombox/cassette/api"
	"github.com/andrebq/boombox/internal/httpserver"
	"github.com/andrebq/boombox/tapedeck"
	"github.com/andrebq/boombox/tapedeck/api"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	bindAddr := "localhost:7008"
	var tapes cli.StringSlice
	return &cli.Command{
		Name:  "query",
		Usage: "Start a boombox api/query instance",
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
		},
		Action: func(ctx *cli.Context) error {
			deck := tapedeck.New()
			defer deck.Close()
			for _, t := range tapes.Value() {
				c, err := cassette.LoadControlCassette(ctx.Context, t, false, true)
				if err != nil {
					return err
				}
				tapeName := filepath.Base(t)
				deck.Load(tapeName[:len(tapeName)-len(filepath.Ext(tapeName))], c)
			}

			toHandler := capi.AsQueryHandler

			handler, err := api.AsHandler(ctx.Context, deck, toHandler)
			if err != nil {
				return err
			}
			return httpserver.Serve(ctx.Context, bindAddr, handler)
		},
	}
}
