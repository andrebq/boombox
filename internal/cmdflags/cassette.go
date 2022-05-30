package cmdflags

import (
	"github.com/andrebq/boombox/cassette/programs/authprogram"
	"github.com/urfave/cli/v2"
)

func Cassette(out *string) cli.Flag {
	return &cli.StringFlag{
		Name:        "cassette",
		Aliases:     []string{"c", "tape", "t"},
		Usage:       "Path to a cassette",
		Destination: out,
		Value:       *out,
	}
}

func RootKeyEnvVar(out *string) cli.Flag {
	if len(*out) == 0 {
		*out = authprogram.RootKeyEnvVar
	}
	return &cli.StringFlag{
		Name:        "root-key-envvar-name",
		Usage:       "Name of the environment variable that holds the root key. The key itself should not be passed as an argument",
		Value:       *out,
		Destination: out,
	}
}
