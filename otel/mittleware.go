package sentryotel

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel/trace"
)

// ContinueFromOtel is a HTTP middleware that can be used with [sentryhttp.Handler] to ensure an existing otel span is
// used as the sentry transaction.
// It should be used whenever the otel tracing is started before the sentry middleware (e.g. to ensure otel sampling
// gets respected across service boundaries)
func ContinueFromOtel(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if otelTrace := trace.SpanFromContext(r.Context()); otelTrace != nil && otelTrace.IsRecording() {
			if transaction, ok := sentrySpanMap.Get(otelTrace.SpanContext().SpanID()); ok {
				r = r.WithContext(sentry.SpanToContext(r.Context(), transaction))
			}
		}
		next.ServeHTTP(w, r)
	})
}
