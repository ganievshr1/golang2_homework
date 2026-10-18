package api

import (
	"log"
	"net/http"
	"time"
)

func Logging(logger *log.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			started := time.Now()

			defer func() {
				logger.Printf(
					"method=%s path=%q duration=%s",
					r.Method,
					r.URL.Path,
					time.Since(started),
				)
			}()

			next.ServeHTTP(w, r)
		})
	}
}
