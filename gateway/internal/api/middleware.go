package api

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
)

const (
	DefaultRequestTimeout = 2 * time.Second
	requestTimeoutEnv = "GATEWAY_REQUEST_TIMEOUT"
	timeoutMessage = "request timeout"
)

type Middleware func(next http.Handler) http.Handler

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func JSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSON)
		next.ServeHTTP(w, r)
	})
}

func RequestTimeout() time.Duration {
	raw := os.Getenv(requestTimeoutEnv)
	if raw == "" {
		return DefaultRequestTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		log.Printf("%s=%q is not a positive duration; using %s", requestTimeoutEnv, raw, DefaultRequestTimeout)
		return DefaultRequestTimeout
	}
	return d
}

// Timeout buffers finite JSON responses. Only this goroutine writes to the client.
// Late handler writes remain in a private buffer and cannot change the 504 response.
// Cancellation of handler work is cooperative: handlers and SQL must observe ctx.
// Streaming, flushing and hijacking are deliberately not supported by this API middleware.
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			if ctx.Err() != nil {
				writeError(w, http.StatusGatewayTimeout, timeoutMessage)
				return
			}
			buffer := httptest.NewRecorder()
			done := make(chan struct{})
			var panicValue any
			go func() {
				defer close(done)
				defer func() {
					if value := recover(); value != nil {
						panicValue = value
						log.Printf("API handler panic: %v", value)
					}
				}()
				next.ServeHTTP(buffer, r.WithContext(ctx))
			}()
			select {
			case <-ctx.Done():
				writeError(w, http.StatusGatewayTimeout, timeoutMessage)
				return
			case <-done:
				// Deadline wins even when both select cases are ready.
				if ctx.Err() != nil {
					writeError(w, http.StatusGatewayTimeout, timeoutMessage)
					return
				}
				if panicValue != nil {
					panic(panicValue)
				}
				for key, values := range buffer.Header() {
					w.Header()[key] = append([]string(nil), values...)
				}
				w.WriteHeader(buffer.Code)
				if _, err := w.Write(buffer.Body.Bytes()); err != nil {
					log.Printf("write API response: %v", err)
				}
			}
		})
	}
}

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
