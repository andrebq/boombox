package private

import (
	"path/filepath"

	"github.com/andrebq/boombox/cassette"
	capi "github.com/andrebq/boombox/cassette/api"
	"github.com/andrebq/boombox/internal/httpserver"
	tplua "github.com/andrebq/boombox/internal/lua/bindings/tapedeck"
	"github.com/andrebq/boombox/tapedeck"
	"github.com/andrebq/boombox/tapedeck/api"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	bindAddr := "localhost:7010"
	var tapes cli.StringSlice
	idxCassette := "index"
	return &cli.Command{
		Name:  "private",
		Usage: "Start a boombox writable instance (ie.: writeable api).",
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
				Usage:       "Cassette name (check cassette flag) to use as the index cassette (ie.: accessible with cassette prefix path)",
				Value:       idxCassette,
				Destination: &idxCassette,
			},
		},
		Action: func(ctx *cli.Context) error {
			deck := tapedeck.New()
			defer deck.Close()
			for _, t := range tapes.Value() {
				c, err := cassette.LoadControlCassette(ctx.Context, t, true, true)
				if err != nil {
					return err
				}
				err = c.EnablePrivileges()
				if err != nil {
					return err
				}
				tapeName := filepath.Base(t)
				deck.Load(tapeName[:len(tapeName)-len(filepath.Ext(tapeName))], c)
			}
			deck.IndexCassette(idxCassette)

			toHandler := capi.AsPrivilegedHandler

			handler, err := api.AsHandler(ctx.Context, deck, tplua.OpenPrivilegedModule(deck), toHandler)
			if err != nil {
				return err
			}
			return httpserver.Serve(ctx.Context, bindAddr, handler)
		},
	}
}
