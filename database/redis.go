package database

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client

func ConnectRedis() {
	dbNumber, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

	options := &redis.Options{
		Addr:     os.Getenv("REDIS_URL"),
		Password: os.Getenv("REDIS_PWD"),
		DB:       dbNumber,
	}
	RedisClient = redis.NewClient(options)
	if err := RedisClient.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}
	log.Println("redis connection successful")
}
