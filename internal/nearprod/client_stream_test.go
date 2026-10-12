package nearprod

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestStreamAPIStopsAfterConsumerCancellation(t *testing.T) {
	for _, tc := range []struct{ name, first, wantCode string }{
		{"line", "event: line\ndata: {\"text\":\"first\"}\n\n", ""},
		{"log-error", "event: log-error\ndata: {\"code\":\"LOG_FAILURE\",\"message\":\"fixture\"}\n\n", "LOG_FAILURE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				// The buffered reader can already contain another event and an
				// incomplete JSON token when the consumer reaches its log cap.
				fmt.Fprint(w, tc.first+"event: line\ndata: {\"text\":\"buffered\"}\n\nevent: line\ndata: {\"text\":")
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer h.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			calls := 0
			err := streamAPI(ctx, J{"url": h.URL, "token": "fixture"}, "/logs", url.Values{}, func(string, J) {
				calls++
				cancel()
			})
			if calls != 1 {
				t.Fatalf("emitted %d events after cancellation; want 1", calls)
			}
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("consumer cancellation decoded an unfinished event: %v", err)
				}
			} else if err == nil || str(publicError(err)["code"]) != tc.wantCode {
				t.Fatalf("stream error lost: %v", err)
			}
		})
	}
}

func TestStreamAPIStillReportsInvalidJSONAndLogErrors(t *testing.T) {
	for _, tc := range []struct{ name, body, wantCode string }{
		{"invalid-json", "event: line\ndata: {invalid}\n\n", "INVALID_JSON"},
		{"incomplete-json", "event: line\ndata: {\"text\":", "INVALID_JSON"},
		{"log-error", "event: log-error\ndata: {\"code\":\"LOG_FAILURE\",\"message\":\"fixture\"}\n\n", "LOG_FAILURE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, tc.body)
			}))
			defer h.Close()
			err := streamAPI(context.Background(), J{"url": h.URL, "token": "fixture"}, "/logs", url.Values{}, func(string, J) {})
			if err == nil || str(publicError(err)["code"]) != tc.wantCode {
				t.Fatalf("want %s, got %v", tc.wantCode, err)
			}
		})
	}
}

func TestStreamAPIConsumerDeadlineRemainsAnError(t *testing.T) {
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: line\ndata: {\"text\":\"first\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := streamAPI(ctx, J{"url": h.URL, "token": "fixture"}, "/logs", url.Values{}, func(string, J) {
		<-ctx.Done()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline was swallowed: %v", err)
	}
}
