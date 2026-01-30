package xrayhq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/event"
)

type mongoMonitor struct {
	mu      sync.Mutex
	started map[int64]mongoStartInfo
}

type mongoStartInfo struct {
	collection string
	operation  string
	filter     string
	startTime  time.Time
	ctx        context.Context
}

// MongoMonitor returns a *event.CommandMonitor that instruments MongoDB operations.
func MongoMonitor() *event.CommandMonitor {
	m := &mongoMonitor{
		started: make(map[int64]mongoStartInfo),
	}
	return &event.CommandMonitor{
		Started:   m.started_,
		Succeeded: m.succeeded,
		Failed:    m.failed,
	}
}

func (m *mongoMonitor) started_(ctx context.Context, evt *event.CommandStartedEvent) {
	collection := ""
	filter := ""

	// Extract collection name from command
	if val, ok := evt.Command.Lookup(evt.CommandName).StringValueOK(); ok {
		collection = val
	}
	// Extract filter if present
	if filterRaw, err := evt.Command.Lookup("filter").DocumentOK(); err {
		filter = filterRaw.String()
	}

	m.mu.Lock()
	m.started[evt.RequestID] = mongoStartInfo{
		collection: collection,
		operation:  evt.CommandName,
		filter:     filter,
		startTime:  time.Now(),
		ctx:        ctx,
	}
	m.mu.Unlock()
}

func (m *mongoMonitor) succeeded(ctx context.Context, evt *event.CommandSucceededEvent) {
	m.mu.Lock()
	info, ok := m.started[evt.RequestID]
	delete(m.started, evt.RequestID)
	m.mu.Unlock()

	if !ok {
		return
	}

	op := MongoOp{
		Collection: info.collection,
		Operation:  info.operation,
		Filter:     info.filter,
		Duration:   time.Since(info.startTime),
		Timestamp:  info.startTime,
	}
	AddMongoOp(info.ctx, op)
}

func (m *mongoMonitor) failed(ctx context.Context, evt *event.CommandFailedEvent) {
	m.mu.Lock()
	info, ok := m.started[evt.RequestID]
	delete(m.started, evt.RequestID)
	m.mu.Unlock()

	if !ok {
		return
	}

	op := MongoOp{
		Collection: info.collection,
		Operation:  info.operation,
		Filter:     info.filter,
		Duration:   time.Since(info.startTime),
		Error:      fmt.Sprintf("%s", evt.Failure),
		Timestamp:  info.startTime,
	}
	AddMongoOp(info.ctx, op)
}
