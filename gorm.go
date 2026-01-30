package xrayhq

import (
	"time"

	"gorm.io/gorm"
)

const gormCallbackName = "xrayhq"

// GORMPlugin implements gorm.Plugin for query instrumentation.
type GORMPlugin struct{}

func NewGORMPlugin() *GORMPlugin {
	return &GORMPlugin{}
}

func (p *GORMPlugin) Name() string {
	return "xrayhq"
}

func (p *GORMPlugin) Initialize(db *gorm.DB) error {
	// Before callbacks - store start time
	db.Callback().Create().Before("gorm:create").Register(gormCallbackName+":before_create", gormBefore)
	db.Callback().Query().Before("gorm:query").Register(gormCallbackName+":before_query", gormBefore)
	db.Callback().Update().Before("gorm:update").Register(gormCallbackName+":before_update", gormBefore)
	db.Callback().Delete().Before("gorm:delete").Register(gormCallbackName+":before_delete", gormBefore)
	db.Callback().Raw().Before("gorm:raw").Register(gormCallbackName+":before_raw", gormBefore)

	// After callbacks - record query
	db.Callback().Create().After("gorm:create").Register(gormCallbackName+":after_create", gormAfter)
	db.Callback().Query().After("gorm:query").Register(gormCallbackName+":after_query", gormAfter)
	db.Callback().Update().After("gorm:update").Register(gormCallbackName+":after_update", gormAfter)
	db.Callback().Delete().After("gorm:delete").Register(gormCallbackName+":after_delete", gormAfter)
	db.Callback().Raw().After("gorm:raw").Register(gormCallbackName+":after_raw", gormAfter)

	return nil
}

type gormStartTimeKey struct{}

func gormBefore(db *gorm.DB) {
	db.Set("xrayhq:start_time", time.Now())
}

func gormAfter(db *gorm.DB) {
	val, ok := db.Get("xrayhq:start_time")
	if !ok {
		return
	}
	startTime, ok := val.(time.Time)
	if !ok {
		return
	}

	duration := time.Since(startTime)
	stmt := db.Statement

	q := DBQuery{
		Query:        stmt.SQL.String(),
		Duration:     duration,
		RowsAffected: stmt.RowsAffected,
		Timestamp:    startTime,
	}
	if db.Error != nil {
		q.Error = db.Error.Error()
	}

	if stmt.Context != nil {
		AddDBQuery(stmt.Context, q)
	}
}
