package telemetry

import (
	"context"
	"time"
)

type SpanKind int

const (
	SpanKindInternal SpanKind = 0
	SpanKindClient   SpanKind = 1
	SpanKindServer   SpanKind = 2
)

type SpanContext struct {
	TraceID string
	SpanID  string
}

type Span struct {
	Name       string
	Context    SpanContext
	Kind       SpanKind
	StartTime  time.Time
	EndTime    time.Time
	Attributes map[string]string
	Events     []SpanEvent
	Status     SpanStatus
}

type SpanStatus struct {
	Code    int
	Message string
}

type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]string
}

type Tracer struct {
	spans []*Span
}

func NewTracer() *Tracer {
	return &Tracer{}
}

func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	span := &Span{
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]string),
	}

	return context.WithValue(ctx, spanKey{}, span), span
}

func (t *Tracer) EndSpan(span *Span) {
	span.EndTime = time.Now()
	t.spans = append(t.spans, span)
}

func (t *Tracer) SetSpanAttribute(span *Span, key, value string) {
	if span.Attributes == nil {
		span.Attributes = make(map[string]string)
	}
	span.Attributes[key] = value
}

func (t *Tracer) AddSpanEvent(span *Span, name string, attrs map[string]string) {
	span.Events = append(span.Events, SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	})
}

func (t *Tracer) SetSpanStatus(span *Span, code int, message string) {
	span.Status = SpanStatus{Code: code, Message: message}
}

func (t *Tracer) Flush() {
	t.spans = nil
}

type spanKey struct{}

func GetSpan(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanKey{}).(*Span); ok {
		return span
	}
	return nil
}

type TraceExporter interface {
	Export(span *Span) error
	Shutdown() error
}

type LogExporter struct{}

func (e *LogExporter) Export(_ *Span) error {
	return nil
}

func (e *LogExporter) Shutdown() error {
	return nil
}

type NoopExporter struct{}

func (e *NoopExporter) Export(_ *Span) error {
	return nil
}

func (e *NoopExporter) Shutdown() error {
	return nil
}
