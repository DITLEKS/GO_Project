package api

import (
	"log"
	"net/http"
	"time"
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

// Chain оборачивает h в middleware; первый в списке — внешний.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
