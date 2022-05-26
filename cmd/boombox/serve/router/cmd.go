package router

import (
	"net/url"

	"github.com/andrebq/boombox/internal/boomproxy"
	"github.com/andrebq/boombox/internal/httpserver"
	"github.com/urfave/cli/v2"
)

func Cmd() *cli.Command {
	bindAddr := "localhost:7007"
	apiEndpoint := "http://locahost:7008/"
	queryEndpoint := "http://localhost:7009/"
	return &cli.Command{
		Name:  "router",
		Usage: "Start the boombox router",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "bind",
				Usage:       "Address to bind for incoming request",
				Destination: &bindAddr,
				Value:       bindAddr,
			},
			&cli.StringFlag{
				Name:        "api-endpoint",
				Usage:       "Base endpoint which contains the API logic",
				Destination: &apiEndpoint,
				Value:       apiEndpoint,
			},
			&cli.StringFlag{
				Name:        "query-endpoint",
				Usage:       "Base endpoint which contains the Query logic",
				Destination: &queryEndpoint,
				Value:       queryEndpoint,
			},
		},
		Action: func(ctx *cli.Context) error {
			apiURL, err := url.Parse(apiEndpoint)
			if err != nil {
				return err
			}
			queryURL, err := url.Parse(queryEndpoint)
			if err != nil {
				return err
			}
			handler := boomproxy.AsHandler(ctx.Context, apiURL, queryURL, nil)
			if err != nil {
				return err
			}
			return httpserver.Serve(ctx.Context, bindAddr, handler)
		},
	}
}
