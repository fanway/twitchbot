package cache

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

var (
	pool        *redis.Pool
	redisServer = "localhost:6379"
)

func newPool(addr string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     100,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}
}

func GetPool() *redis.Pool {
	return pool
}

func init() {
	pool = newPool(redisServer)
}
