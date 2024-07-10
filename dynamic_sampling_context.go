package sentry

import (
	"strconv"
	"strings"

	"github.com/exaring/sentry-go/internal/otel/baggage"
)

const (
	sentryPrefix = "sentry-"
)

// DynamicSamplingContext holds information about the current event that can be used to make dynamic sampling decisions.
type DynamicSamplingContext struct {
	Entries map[string]string
	Frozen  bool
}

func DynamicSamplingContextFromHeader(header []byte) (DynamicSamplingContext, error) {
	bag, err := baggage.Parse(string(header))
	if err != nil {
		return DynamicSamplingContext{}, err
	}

	entries := map[string]string{}
	for _, member := range bag.Members() {
		// We only store baggage members if their key starts with "sentry-".
		if k, v := member.Key(), member.Value(); strings.HasPrefix(k, sentryPrefix) {
			entries[strings.TrimPrefix(k, sentryPrefix)] = v
		}
	}

	return DynamicSamplingContext{
		Entries: entries,
		// If there's at least one Sentry value, we consider the DSC frozen
		Frozen: len(entries) > 0,
	}, nil
}

func DynamicSamplingContextFromTransaction(span *Span) DynamicSamplingContext {
	entries := map[string]string{}

	hub := hubFromContext(span.Context())
	scope := hub.Scope()
	client := hub.Client()

	if client == nil || scope == nil {
		return DynamicSamplingContext{
			Entries: map[string]string{},
			Frozen:  false,
		}
	}

	if traceID := span.TraceID.String(); traceID != "" {
		entries["trace_id"] = traceID
	}
	if sampleRate := span.sampleRate; sampleRate != 0 {
		entries["sample_rate"] = strconv.FormatFloat(sampleRate, 'f', -1, 64)
	}

	if dsn := client.dsn; dsn != nil {
		if publicKey := dsn.publicKey; publicKey != "" {
			entries["public_key"] = publicKey
		}
	}
	if release := client.options.Release; release != "" {
		entries["release"] = release
	}
	if environment := client.options.Environment; environment != "" {
		entries["environment"] = environment
	}

	// Only include the transaction name if it's of good quality (not empty and not SourceURL)
	if span.Source != "" && span.Source != SourceURL {
		if span.IsTransaction() {
			entries["transaction"] = span.Name
		}
	}

	entries["sampled"] = strconv.FormatBool(span.Sampled.Bool())

	return DynamicSamplingContext{
		Entries: entries,
		Frozen:  true,
	}
}

func (d DynamicSamplingContext) HasEntries() bool {
	return len(d.Entries) > 0
}

func (d DynamicSamplingContext) IsFrozen() bool {
	return d.Frozen
}

func (d DynamicSamplingContext) String() string {
	members := []baggage.Member{}
	for k, entry := range d.Entries {
		member, err := baggage.NewMember(sentryPrefix+k, entry)
		if err != nil {
			continue
		}
		members = append(members, member)
	}
	if len(members) > 0 {
		baggage, err := baggage.New(members...)
		if err != nil {
			return ""
		}
		return baggage.String()
	}

	return ""
}

// Constructs a new DynamicSamplingContext using a scope and client. Accessing
// fields on the scope are not thread safe, and this function should only be
// called within scope methods.
func DynamicSamplingContextFromScope(scope *Scope, client *Client) DynamicSamplingContext {
	entries := map[string]string{}

	if client == nil || scope == nil {
		return DynamicSamplingContext{
			Entries: entries,
			Frozen:  false,
		}
	}

	propagationContext := scope.propagationContext

	if traceID := propagationContext.TraceID.String(); traceID != "" {
		entries["trace_id"] = traceID
	}
	if sampleRate := client.options.TracesSampleRate; sampleRate != 0 {
		entries["sample_rate"] = strconv.FormatFloat(sampleRate, 'f', -1, 64)
	}

	if dsn := client.dsn; dsn != nil {
		if publicKey := dsn.publicKey; publicKey != "" {
			entries["public_key"] = publicKey
		}
	}
	if release := client.options.Release; release != "" {
		entries["release"] = release
	}
	if environment := client.options.Environment; environment != "" {
		entries["environment"] = environment
	}

	return DynamicSamplingContext{
		Entries: entries,
		Frozen:  true,
	}
}
