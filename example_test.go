package redigo_rate_test

import (
	"context"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/iam5j/redigo_rate"
)

func ExampleNewLimiter() {
	ctx := context.Background()
	rdb := &redis.Pool{
		MaxActive:   5,
		MaxIdle:     5,
		IdleTimeout: 5 * time.Minute,
		Wait:        true,
		DialContext: func(ctx context.Context) (conn redis.Conn, err error) {
			conn, err = redis.DialURLContext(ctx, "redis://127.0.0.1:6379")
			return conn, err
		},
	}

	c, err := rdb.GetContext(ctx)
	if err != nil {
		panic(err)
	}

	defer c.Close()
	err = c.Flush()
	if err != nil {
		panic(err)
	}

	limiter := redigo_rate.NewLimiter(rdb)
	res, err := limiter.Allow(ctx, "project:123", redigo_rate.PerSecond(10))
	if err != nil {
		panic(err)
	}

	fmt.Println("allowed", res.Allowed, "remaining", res.Remaining)
	// Output: allowed 1 remaining 9
}
