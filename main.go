package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
)

func main() {
	redisServer := os.Getenv("REDIS_SERVER")
	redisCStr := fmt.Sprintf("%s:6379", redisServer)
	redisClient := redis.NewClient(&redis.Options{Addr: redisCStr, Password: ""})

	brokerRouter := mux.NewRouter()
	brokerRouter.HandleFunc("/register_service", func(w http.ResponseWriter, r *http.Request) {
		pong, err := redisClient.Ping().Result()
		fmt.Fprintln(w, pong, err)
	})

	brokerRouter.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	s := &http.Server{
		Addr:    ":80",
		Handler: brokerRouter,
	}
	log.Fatal(s.ListenAndServe())
}
