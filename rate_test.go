package redigo_rate_test

import (
	"context"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/stretchr/testify/require"

	"github.com/iam5j/redigo_rate"
)

func rateLimiter(ctx context.Context) *redigo_rate.Limiter {
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

	return redigo_rate.NewLimiter(rdb)
}

func TestAllow(t *testing.T) {
	ctx := context.Background()

	l := rateLimiter(ctx)

	limit := redigo_rate.PerSecond(10)
	require.Equal(t, limit.String(), "10 req/s (burst 10)")
	require.False(t, limit.IsZero())

	res, err := l.Allow(ctx, "test_id", limit)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 1)
	require.Equal(t, res.Remaining, 9)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 100*time.Millisecond, float64(10*time.Millisecond))

	err = l.Reset(ctx, "test_id")
	require.Nil(t, err)
	res, err = l.Allow(ctx, "test_id", limit)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 1)
	require.Equal(t, res.Remaining, 9)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 100*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowN(ctx, "test_id", limit, 2)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 2)
	require.Equal(t, res.Remaining, 7)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 300*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowN(ctx, "test_id", limit, 7)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 7)
	require.Equal(t, res.Remaining, 0)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 999*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowN(ctx, "test_id", limit, 1000)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 0)
	require.InDelta(t, res.RetryAfter, 99*time.Second, float64(time.Second))
	require.InDelta(t, res.ResetAfter, 999*time.Millisecond, float64(10*time.Millisecond))
}

func TestAllowN_IncrementZero(t *testing.T) {
	ctx := context.Background()
	l := rateLimiter(ctx)
	limit := redigo_rate.PerSecond(10)

	// Check for a row that's not there
	res, err := l.AllowN(ctx, "test_id", limit, 0)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 10)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.Equal(t, res.ResetAfter, time.Duration(0))

	// Now increment it
	res, err = l.Allow(ctx, "test_id", limit)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 1)
	require.Equal(t, res.Remaining, 9)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 100*time.Millisecond, float64(10*time.Millisecond))

	// Peek again
	res, err = l.AllowN(ctx, "test_id", limit, 0)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 9)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 100*time.Millisecond, float64(10*time.Millisecond))
}

func TestRetryAfter(t *testing.T) {
	limit := redigo_rate.Limit{
		Rate:   1,
		Period: time.Millisecond,
		Burst:  1,
	}

	ctx := context.Background()
	l := rateLimiter(ctx)

	for i := 0; i < 1000; i++ {
		res, err := l.Allow(ctx, "test_id", limit)
		require.Nil(t, err)

		if res.Allowed > 0 {
			continue
		}

		require.LessOrEqual(t, int64(res.RetryAfter), int64(time.Millisecond))
	}
}

func TestAllowAtMost(t *testing.T) {
	ctx := context.Background()

	l := rateLimiter(ctx)
	limit := redigo_rate.PerSecond(10)

	res, err := l.Allow(ctx, "test_id", limit)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 1)
	require.Equal(t, res.Remaining, 9)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 100*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowAtMost(ctx, "test_id", limit, 2)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 2)
	require.Equal(t, res.Remaining, 7)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 300*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowN(ctx, "test_id", limit, 0)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 7)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 300*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowAtMost(ctx, "test_id", limit, 10)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 7)
	require.Equal(t, res.Remaining, 0)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 999*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowN(ctx, "test_id", limit, 0)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 0)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 999*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowAtMost(ctx, "test_id", limit, 1000)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 0)
	require.InDelta(t, res.RetryAfter, 99*time.Millisecond, float64(10*time.Millisecond))
	require.InDelta(t, res.ResetAfter, 999*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowN(ctx, "test_id", limit, 1000)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 0)
	require.InDelta(t, res.RetryAfter, 99*time.Second, float64(time.Second))
	require.InDelta(t, res.ResetAfter, 999*time.Millisecond, float64(10*time.Millisecond))
}

func TestAllowAtMost_IncrementZero(t *testing.T) {
	ctx := context.Background()
	l := rateLimiter(ctx)
	limit := redigo_rate.PerSecond(10)

	// Check for a row that isn't there
	res, err := l.AllowAtMost(ctx, "test_id", limit, 0)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 10)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.Equal(t, res.ResetAfter, time.Duration(0))

	// Now increment it
	res, err = l.Allow(ctx, "test_id", limit)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 1)
	require.Equal(t, res.Remaining, 9)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 100*time.Millisecond, float64(10*time.Millisecond))

	// Peek again
	res, err = l.AllowAtMost(ctx, "test_id", limit, 0)
	require.Nil(t, err)
	require.Equal(t, res.Allowed, 0)
	require.Equal(t, res.Remaining, 9)
	require.Equal(t, res.RetryAfter, time.Duration(-1))
	require.InDelta(t, res.ResetAfter, 100*time.Millisecond, float64(10*time.Millisecond))
}

func BenchmarkAllow(b *testing.B) {
	ctx := context.Background()
	l := rateLimiter(ctx)
	limit := redigo_rate.PerSecond(1e6)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res, err := l.Allow(ctx, "foo", limit)
			if err != nil {
				b.Fatal(err)
			}
			if res.Allowed == 0 {
				panic("not reached")
			}
		}
	})
}

func BenchmarkAllowAtMost(b *testing.B) {
	ctx := context.Background()
	l := rateLimiter(ctx)
	limit := redigo_rate.PerSecond(1e6)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res, err := l.AllowAtMost(ctx, "foo", limit, 1)
			if err != nil {
				b.Fatal(err)
			}
			if res.Allowed == 0 {
				panic("not reached")
			}
		}
	})
}
