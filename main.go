package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
)

type registerServiceData struct {
	HelmChartName string `json:"helm_chart_name"`
}

func main() {
	redisServer := os.Getenv("REDIS_SERVER")
	redisCStr := fmt.Sprintf("%s:6379", redisServer)
	redisClient := redis.NewClient(&redis.Options{Addr: redisCStr, Password: ""})

	brokerRouter := mux.NewRouter()
	brokerRouter.HandleFunc("/register-service/{name}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		name := vars["name"]
		if name == "" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "name is empty")
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err.Error())
			return
		}
		defer r.Body.Close()

		j := registerServiceData{}
		if err = json.Unmarshal(body, &j); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, err.Error())
			return
		}
		if j.HelmChartName == "" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, "helmChartName is empty")
			return
		}

		status := redisClient.Set(name, j.HelmChartName, 0)
		if status.Err() != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, status.Err().Error())
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, "added %s: %s", name, j.HelmChartName)
	}).Methods("POST")

	brokerRouter.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := redisClient.Ping().Result(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}).Methods("GET")

	s := &http.Server{
		Addr:    ":80",
		Handler: brokerRouter,
	}
	log.Fatal(s.ListenAndServe())
}
