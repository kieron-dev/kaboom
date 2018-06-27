package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	brokerRouter := mux.NewRouter()
	brokerRouter.HandleFunc("/register_service", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, world!")
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
