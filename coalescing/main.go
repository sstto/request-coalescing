package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sees/coalescing/core"
	"sees/coalescing/middleware"
	"time"
)

const (
	baseURL = "https://httpbin.org"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/request",
		handleRequest,
	)
	coalescingProcessor := core.NewCoalescingProcessor(3 * time.Second)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           middleware.RequestCoalescing(coalescingProcessor, mux),
		IdleTimeout:       time.Minute,
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		for {
			if <-c == os.Interrupt {
				_ = srv.Close()
				return
			}
		}
	}()

	err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
		return
	}

	log.Println("Server stopped")
}

func handleRequest(w http.ResponseWriter, req *http.Request) {
	log.Printf("handleRequest %s\n", req.URL)
	response := sendRequest(baseURL)
	if response != nil {
		body, _ := io.ReadAll(response.Body)
		w.WriteHeader(response.StatusCode)
		_, err := w.Write(body)
		if err != nil {
			log.Fatal(err)
			return
		}
	}
}

func sendRequest(URL string) *http.Response {
	log.Printf("Sending request to %s\n", URL)
	resp, err := http.Get(URL)
	if err != nil {
		log.Fatal("Error sending request: ", err)
		return nil
	}
	return resp
}
