package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

const (
	baseURL       = "https://httpbin.org"
	coalesceDelay = 5 * time.Second
)

var (
	coalesceMap  = make(map[string]*RequestData)
	coalesceLock sync.Mutex
)

type RequestData struct {
	URL     string
	Counter int
	Results chan *ResponseData
}

type ResponseData struct {
	StatusCode int
	Body       []byte
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/request",
		handleRequest,
	)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
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

	go processCoalesceRequest()

	err := srv.ListenAndServe()
	if err != nil {
		log.Fatal(err)
		return
	}

	log.Println("Server stopped")
}

func handleRequest(w http.ResponseWriter, req *http.Request) {
	log.Printf("handleRequest %s\n", req.URL)
	URL := fmt.Sprintf("%s/get", baseURL)

	coalesceLock.Lock()
	data, exists := coalesceMap[URL]
	if !exists {
		data = &RequestData{
			URL:     URL,
			Counter: 1,
			Results: make(chan *ResponseData),
		}
		coalesceMap[URL] = data
	} else {
		data.Counter++
	}
	coalesceLock.Unlock()

	response := <-data.Results

	if response != nil {
		w.WriteHeader(response.StatusCode)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(response.Body)
		if err != nil {
			log.Fatal(err)
			return
		}
	}

}

func processCoalesceRequest() {
	for {
		time.Sleep(coalesceDelay)

		log.Println("Processing coalesce queue")
		coalesceLock.Lock()
		for _, data := range coalesceMap {
			go sendCoalesceRequest(data)
		}
		coalesceLock.Unlock()
	}
}

func sendCoalesceRequest(data *RequestData) {
	if data.Counter == 0 {
		return
	}
	log.Printf("Coalescing %d requests for URL: %s\n\n", data.Counter, data.URL)
	resp := sendRequest(data.URL)
	for data.Counter > 0 {
		log.Println(data.Counter)
		data.Results <- resp
		data.Counter--
	}
}

func sendRequest(URL string) *ResponseData {
	log.Printf("Sending request to %s\n", URL)
	resp, err := http.Get(URL)
	if err != nil {
		log.Fatal("Error sending request: ", err)
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	return &ResponseData{
		StatusCode: resp.StatusCode,
		Body:       body,
	}
}
