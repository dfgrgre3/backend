package redisconn

import (
	"log"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

// ParseAsynqConnOpt returns tuned asynq.RedisConnOpt with safe network timeouts (5s)
// and pool size (20) to prevent i/o timeout error cascades in worker processes under load.
func ParseAsynqConnOpt(redisAddr string) asynq.RedisConnOpt {
	if redisAddr == "" {
		return asynq.RedisClientOpt{}
	}

	if strings.HasPrefix(redisAddr, "redis://") || strings.HasPrefix(redisAddr, "rediss://") {
		parsedOpts, err := asynq.ParseRedisURI(redisAddr)
		if err != nil {
			log.Printf("[Cache][Worker] failed to parse redis uri %q: %v", redisAddr, err)
			return asynq.RedisClientOpt{Addr: redisAddr}
		}

		switch opt := parsedOpts.(type) {
		case asynq.RedisClientOpt:
			opt.DialTimeout = 5 * time.Second
			opt.ReadTimeout = 5 * time.Second
			opt.WriteTimeout = 5 * time.Second
			opt.PoolSize = 20
			return opt
		case asynq.RedisClusterClientOpt:
			opt.DialTimeout = 5 * time.Second
			opt.ReadTimeout = 5 * time.Second
			opt.WriteTimeout = 5 * time.Second
			return opt
		default:
			return parsedOpts
		}
	}

	return asynq.RedisClientOpt{
		Addr:         redisAddr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolSize:     20,
	}
}
