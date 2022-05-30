package authprogram

import (
	"bufio"
	"crypto/rand"
	"errors"
	"os"
	"strings"

	"github.com/andrebq/boombox/cassette"
	"github.com/andrebq/boombox/cassette/programs/authprogram"
	"github.com/andrebq/boombox/internal/cmdflags"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	var tape *cassette.Control
	var keyfn authprogram.KeyFn
	var tapeName string
	var rootKeyEnvVar string
	return &cli.Command{
		Name:  "auth",
		Usage: "Auth program performs functions on cassettes that have it installed.",
		Flags: []cli.Flag{
			cmdflags.Cassette(&tapeName),
			cmdflags.RootKeyEnvVar(&rootKeyEnvVar),
		},
		Before: func(ctx *cli.Context) error {
			var err error
			tape, err = cassette.LoadControlCassette(ctx.Context, tapeName, true, true)
			if err != nil {
				return err
			}
			err = tape.EnablePrivileges()
			if err != nil {
				return err
			}
			keyfn, err = authprogram.KeyFNFromEnv(rootKeyEnvVar, os.Getenv, os.Setenv)
			if err != nil {
				return err
			}
			return nil
		},
		Subcommands: []*cli.Command{
			registerCmd(&tape, &keyfn),
		},
	}
}

func registerCmd(tape **cassette.Control, keyfn *authprogram.KeyFn) *cli.Command {
	var username string
	var password string
	return &cli.Command{
		Name:  "register",
		Usage: "Register a new user in the given cassette (password is read from stdin)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "username",
				Aliases:     []string{"u", "user"},
				Usage:       "Name of the user to register",
				Destination: &username,
				Required:    true,
			},
		},
		Action: func(ctx *cli.Context) error {
			sc := bufio.NewScanner(os.Stdin)
			if !sc.Scan() {
				return sc.Err()
			}
			password = strings.TrimSpace(sc.Text())
			if len(password) == 0 {
				return errors.New("missing password from stdin")
			}
			err := authprogram.Setup(ctx.Context, *tape)
			if err != nil {
				return err
			}
			err = authprogram.Register(ctx.Context, *tape,
				authprogram.PlainText(username), authprogram.PlainText(password), *keyfn, rand.Reader)
			return err
		},
	}
}
