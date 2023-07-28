package sentryotel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"go.opentelemetry.io/otel"
	otelSdkTrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func emptyContextWithSentryAndTracing(t *testing.T) (context.Context, map[string]*sentry.Span) {
	t.Helper()

	// we want to check sent events after they're finished, so sentrySpanMap cannot be used
	spans := make(map[string]*sentry.Span)

	client, err := sentry.NewClient(sentry.ClientOptions{
		Debug:         true,
		Dsn:           "https://abc@example.com/123",
		Environment:   "testing",
		Release:       "1.2.3",
		EnableTracing: true,
		BeforeSendTransaction: func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
			for _, span := range event.Spans {
				spans[span.SpanID.String()] = span
			}
			return event
		},
	})
	if err != nil {
		t.Fatalf("failed to create sentry client: %v", err)
	}

	hub := sentry.NewHub(client, sentry.NewScope())
	return sentry.SetHubOnContext(context.Background(), hub), spans
}

func TestRespectOtelSampling(t *testing.T) {
	spanProcessor := NewSentrySpanProcessor()

	simulateOtelAndSentry := func(ctx context.Context) (root, inner trace.Span) {
		handler := sentryhttp.New(sentryhttp.Options{}).Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, inner = otel.Tracer("").Start(r.Context(), "test-inner-span")
			defer inner.End()
		}))
		handler = ContinueFromOtel(handler)

		tracer := otel.Tracer("")
		// simulate an otel middleware creating the root span before sentry
		ctx, root = tracer.Start(ctx, "test-root-span")
		defer root.End()

		handler.ServeHTTP(
			httptest.NewRecorder(),
			httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx),
		)

		return root, inner
	}

	t.Run("always sample", func(t *testing.T) {
		tp := otelSdkTrace.NewTracerProvider(
			otelSdkTrace.WithSpanProcessor(spanProcessor),
			otelSdkTrace.WithSampler(otelSdkTrace.AlwaysSample()),
		)
		otel.SetTracerProvider(tp)

		ctx, spans := emptyContextWithSentryAndTracing(t)

		root, inner := simulateOtelAndSentry(ctx)

		if root.SpanContext().TraceID() != inner.SpanContext().TraceID() {
			t.Errorf("otel root span and inner span should have the same trace id")
		}

		if len(spans) != 1 {
			t.Errorf("got unexpected number of events sent to sentry: %d != 1", len(spans))
		}

		for _, span := range []trace.Span{root, inner} {
			if !span.SpanContext().IsSampled() {
				t.Errorf("otel span should be sampled")
			}
		}

		// the root span is encoded into the event's context, not in sentry.Event.Spans
		spanID := inner.SpanContext().SpanID().String()
		sentrySpan, ok := spans[spanID]
		if !ok {
			t.Fatalf("sentry event could not be found from otel span %s", spanID)
		}

		if sentrySpan.Sampled != sentry.SampledTrue {
			t.Errorf("sentry span should be sampled, not %v", sentrySpan.Sampled)
		}
	})

	t.Run("never sample", func(t *testing.T) {
		tp := otelSdkTrace.NewTracerProvider(
			otelSdkTrace.WithSpanProcessor(spanProcessor),
			otelSdkTrace.WithSampler(otelSdkTrace.NeverSample()),
		)
		otel.SetTracerProvider(tp)

		ctx, spans := emptyContextWithSentryAndTracing(t)

		root, inner := simulateOtelAndSentry(ctx)

		if len(spans) != 0 {
			t.Fatalf("sentry span should not have been sent to sentry")
		}

		for _, span := range []trace.Span{root, inner} {
			if span.SpanContext().IsSampled() {
				t.Errorf("otel span should not be sampled")
			}
		}
	})
}
