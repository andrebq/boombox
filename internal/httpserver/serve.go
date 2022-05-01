package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/andrebq/boombox/internal/logutil"
)

func Serve(ctx context.Context, bind string, handler http.Handler) error {
	server := http.Server{
		Handler:           handler,
		Addr:              bind,
		ReadTimeout:       time.Minute * 5,
		WriteTimeout:      time.Minute,
		ReadHeaderTimeout: time.Minute,
		IdleTimeout:       time.Minute * 5,
	}
	err := make(chan error, 1)
	done := make(chan struct{})
	go serveInBackground(ctx, &server, err, done)
	<-done
	return <-err
}

func serveInBackground(ctx context.Context, server *http.Server, firstErr chan<- error, done chan<- struct{}) {
	log := logutil.GetOrDefault(ctx)
	defer close(done)
	serverCtx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		defer close(firstErr)
		log.Info().Str("server.addr", server.Addr).Msg("Starting HTTP server")
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			// shutdown called,
			// ignore the error
			return
		} else if err != nil {
			select {
			case firstErr <- err:
			default:
			}
			return
		}
	}()
	select {
	case <-serverCtx.Done():
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), time.Minute)
		defer cancelShutdown()
		server.Shutdown(shutdownCtx)
	}
}
