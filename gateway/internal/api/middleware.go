package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	// DefaultRequestTimeout is the time limit for one API request.
	DefaultRequestTimeout = 2 * time.Second
	// requestTimeoutEnv overrides the default; the value is a Go duration, e.g. "500ms" or "3s".
	requestTimeoutEnv = "GATEWAY_REQUEST_TIMEOUT"
	// timeoutMessage is returned with 504 Gateway Timeout when a request is cancelled or times out.
	timeoutMessage = "request timeout"
)

// Middleware — обёртка над http.Handler.
type Middleware func(next http.Handler) http.Handler

// Logging логирует метод, путь и длительность обработки каждого запроса.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// JSONContentType добавляет ко всем ответам единый Content-Type.
func JSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentTypeJSON)
		next.ServeHTTP(w, r)
	})
}

// RequestTimeout возвращает таймаут запроса: GATEWAY_REQUEST_TIMEOUT или значение по умолчанию.
// Некорректное или неположительное значение игнорируется.
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

// Timeout создаёт для каждого запроса context.WithTimeout и передаёт его дальше через *http.Request.
// Обработчик получает ctx из r.Context(); сервис и SQL-запросы прерываются по ctx.Done(),
// после чего обработчик отвечает 504 и {"error":"request timeout"}.
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Chain оборачивает h в middleware; первый в списке — внешний.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
