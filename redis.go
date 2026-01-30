package xrayhq

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisHook struct{}

// RedisHook returns a go-redis Hook that instruments Redis commands.
func RedisHook() redis.Hook {
	return &redisHook{}
}

type redisStartKey struct{}

func (h *redisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *redisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		duration := time.Since(start)

		args := cmd.Args()
		command := ""
		key := ""
		if len(args) > 0 {
			command = strings.ToUpper(args[0].(string))
		}
		if len(args) > 1 {
			if k, ok := args[1].(string); ok {
				key = k
			}
		}

		op := RedisOp{
			Command:   command,
			Key:       key,
			Duration:  duration,
			Timestamp: start,
		}
		if err != nil {
			op.Error = err.Error()
		}
		AddRedisOp(ctx, op)
		return err
	}
}

func (h *redisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		duration := time.Since(start)

		for _, cmd := range cmds {
			args := cmd.Args()
			command := ""
			key := ""
			if len(args) > 0 {
				command = strings.ToUpper(args[0].(string))
			}
			if len(args) > 1 {
				if k, ok := args[1].(string); ok {
					key = k
				}
			}
			op := RedisOp{
				Command:   command,
				Key:       key,
				Duration:  duration / time.Duration(len(cmds)),
				Timestamp: start,
			}
			if err != nil {
				op.Error = err.Error()
			}
			AddRedisOp(ctx, op)
		}
		return err
	}
}
