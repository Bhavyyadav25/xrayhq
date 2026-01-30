package xrayhq

import "context"

type contextKey struct{ name string }

var traceKey = &contextKey{"xrayhq-trace"}

func withTrace(ctx context.Context, t *RequestTrace) context.Context {
	return context.WithValue(ctx, traceKey, t)
}

func TraceFromContext(ctx context.Context) *RequestTrace {
	t, _ := ctx.Value(traceKey).(*RequestTrace)
	return t
}

func AddDBQuery(ctx context.Context, q DBQuery) {
	t := TraceFromContext(ctx)
	if t == nil {
		return
	}
	t.DBQueries = append(t.DBQueries, q)
	t.TotalDBTime += q.Duration
}

func AddExternalCall(ctx context.Context, c ExternalCall) {
	t := TraceFromContext(ctx)
	if t == nil {
		return
	}
	t.ExternalCalls = append(t.ExternalCalls, c)
	t.TotalExtTime += c.Duration
}

func AddRedisOp(ctx context.Context, op RedisOp) {
	t := TraceFromContext(ctx)
	if t == nil {
		return
	}
	t.RedisOps = append(t.RedisOps, op)
	t.TotalRedisTime += op.Duration
}

func AddMongoOp(ctx context.Context, op MongoOp) {
	t := TraceFromContext(ctx)
	if t == nil {
		return
	}
	t.MongoOps = append(t.MongoOps, op)
	t.TotalMongoTime += op.Duration
}
