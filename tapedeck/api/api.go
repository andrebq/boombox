package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/andrebq/boombox/cassette/api"
	"github.com/andrebq/boombox/tapedeck"
)

func AsHandler(ctx context.Context, d *tapedeck.D) (http.Handler, error) {
	cassettes := d.List()
	mux := http.NewServeMux()
	for _, c := range cassettes {
		prefix := fmt.Sprintf("/%v", c)
		handler, err := api.AsHandler(ctx, d.Get(c))
		if err != nil {
			return nil, err
		}
		mux.Handle(fmt.Sprintf("%v/", prefix), http.StripPrefix(prefix, handler))
	}
	idx := d.Index()
	if idx != nil {
		idxHandler, err := api.AsHandler(ctx, idx)
		if err != nil {
			return nil, err
		}
		mux.Handle("/", idxHandler)
	}
	return mux, nil
}
