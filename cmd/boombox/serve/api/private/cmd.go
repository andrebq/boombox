package private

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/andrebq/boombox/cassette"
	capi "github.com/andrebq/boombox/cassette/api"
	"github.com/andrebq/boombox/cassette/programs/authprogram"
	authapi "github.com/andrebq/boombox/cassette/programs/authprogram/api"
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
	authCassette := ""
	authKeyEnvVarName := authprogram.RootKeyEnvVar
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
			&cli.StringFlag{
				Name:        "auth",
				Usage:       "Name of the cassette that contains the auth program (leave empty to use the index one)",
				Value:       authCassette,
				Destination: &authCassette,
			},
			&cli.StringFlag{
				Name:        "root-key-envvar-name",
				Usage:       "Name of the environment variable that holds the root key. The key itself should not be passed as an argument",
				Hidden:      true,
				Value:       authKeyEnvVarName,
				Destination: &authKeyEnvVarName,
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
			if authCassette == "" {
				authCassette = idxCassette
			}
			deck.AuthCassette(authCassette)

			if deck.Index() == nil {
				return errors.New("cannot run the private api without an index cassette")
			}

			if deck.Auth() == nil {
				return errors.New("cannot run the private api without an auth cassette")
			}

			toHandler := capi.AsPrivilegedHandler

			handler, err := api.AsHandler(ctx.Context, deck, tplua.OpenPrivilegedModule(deck), toHandler)
			if err != nil {
				return err
			}
			keyfn, err := authprogram.KeyFNFromEnv(authKeyEnvVarName, os.Getenv, os.Setenv)
			if err != nil {
				return err
			}
			realm := authapi.NewRealm(deck.Auth(), authprogram.InMemoryTokenStore(), keyfn, false)
			return httpserver.Serve(ctx.Context, bindAddr, realm.Protect(handler))
		},
	}
}
