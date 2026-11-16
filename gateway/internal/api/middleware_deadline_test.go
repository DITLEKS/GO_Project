package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTimeoutDiscardsLateSuccess(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})
	h := Timeout(20*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		<-release // Deliberately ignore cancellation until the test releases us.
		w.Header().Set("X-Late", "yes")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"late":true}`))
	}))
	rec := httptest.NewRecorder()
	returned := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/test", nil))
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2*time.Second):
		close(release)
		<-finished
		t.Fatal("middleware waited for a handler ignoring cancellation")
	}
	close(release)
	<-finished
	requireError(t, rec, http.StatusGatewayTimeout, timeoutMessage)
	if rec.Header().Get("X-Late") != "" || strings.Contains(rec.Body.String(), "late") {
		t.Fatal("late response leaked to client")
	}
}

func TestTimeoutPreservesSuccessfulResponse(t *testing.T) {
	h := Timeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.Context().Deadline(); !ok {
			panic("missing request deadline")
		}
		w.Header().Set("X-Test", "ok")
		writeJSON(w, http.StatusCreated, map[string]bool{"ok":true})
	}))
	rec := doRequest(h, "GET", "/api/test", "")
	requireJSON(t, rec, http.StatusCreated)
	if rec.Header().Get("X-Test") != "ok" || strings.TrimSpace(rec.Body.String()) != `{"ok":true}` {
		t.Fatal(rec.Header(), rec.Body.String())
	}
}

func TestTimeoutAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	h := Timeout(time.Second)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/test", nil).WithContext(ctx))
	requireError(t, rec, http.StatusGatewayTimeout, timeoutMessage)
	if called {
		t.Fatal("handler started with an already cancelled context")
	}
}
