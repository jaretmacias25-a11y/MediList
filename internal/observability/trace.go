// Package observability contiene primitivas compartidas de trazabilidad.
// No depende de la capa HTTP para que los servicios de negocio puedan
// recuperar el trace_id desde el contexto sin invertir dependencias.
package observability

import "context"

type traceIDKey struct{}

// WithTraceID agrega el identificador de correlación al contexto.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// TraceID devuelve el identificador de correlación del contexto, o "".
func TraceID(ctx context.Context) string {
	traceID, _ := ctx.Value(traceIDKey{}).(string)
	return traceID
}
