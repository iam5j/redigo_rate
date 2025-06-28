# Rate limiting for redigo

[![Build Status](https://travis-ci.org/iam5j/redigo_rate.svg?branch=master)](https://travis-ci.org/iam5j/redigo_rate)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/gomodule/redigo/redis)](https://pkg.go.dev/github.com/iam5j/redigo_rate)

> :heart: [**Uptrace.dev** - distributed traces, logs, and errors in one place](https://uptrace.dev)

This package is inspired by [redis_rate](https://github.com/go-redis/redis_rate).
It based on [rwz/redis-gcra](https://github.com/rwz/redis-gcra) and implements
[GCRA](https://en.wikipedia.org/wiki/Generic_cell_rate_algorithm) (aka leaky bucket) for rate
limiting based on Redis. The code requires Redis version 3.2 or newer since it relies on
[replicate_commands](https://redis.io/commands/eval#replicating-commands-instead-of-scripts)
feature.

## Installation

redis_rate supports 2 last Go versions and requires a Go version with
[modules](https://github.com/golang/go/wiki/Modules) support. So make sure to initialize a Go
module:

```shell
go mod init github.com/my/repo
```

And then install redigo:

```shell
go get github.com/gomodule/redigo/redis
```

## Example

```go
package redigo_rate_test

import (
	"context"
	"fmt"

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
```
