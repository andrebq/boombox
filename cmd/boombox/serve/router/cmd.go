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
	adminEndpoint := "http://localhost:7010/admin"
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
			&cli.StringFlag{
				Name:        "admin-endpoint",
				Usage:       "Base endpoint which hosts the admin interface (the only one with write permissions)",
				Destination: &adminEndpoint,
				Value:       adminEndpoint,
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
			var adminURL *url.URL
			if adminEndpoint != "" {
				adminURL, err = url.Parse(adminEndpoint)
				if err != nil {
					return err
				}
			}
			handler, err := boomproxy.AsHandler(ctx.Context, apiURL, queryURL, adminURL)
			if err != nil {
				return err
			}
			return httpserver.Serve(ctx.Context, bindAddr, handler)
		},
	}
}
