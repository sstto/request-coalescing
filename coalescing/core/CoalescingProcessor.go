package core

import (
	"log"
	"net/http"
	"sync"
	"time"
)

type CoalescingProcessor interface {
	AddRequest(request *http.Request, next http.Handler) *RequestData
	Start()
}

type coalescingProcessor struct {
	interval     time.Duration
	coalesceMap  map[string]*RequestData
	coalesceLock sync.Mutex
}

type RequestData struct {
	URL     string
	Counter int
	Next    http.Handler
	Request *http.Request
	Results chan *ResponseData
}

type ResponseData struct {
	StatusCode int
	Body       []byte
}

type responseRecorder struct {
	http.ResponseWriter
	statusCodeChannel chan int
	bodyChannel       chan []byte
}

func (rec *responseRecorder) WriteHeader(s int) {
	log.Println("Writing Header")
	rec.statusCodeChannel <- s
}

func (rec *responseRecorder) Write(b []byte) (int, error) {
	log.Println("Writing response")
	rec.bodyChannel <- b
	return len(b), nil
}

func NewCoalescingProcessor(interval time.Duration) CoalescingProcessor {
	p := &coalescingProcessor{
		interval: interval,
	}
	p.Start()
	return p
}

func (p *coalescingProcessor) AddRequest(request *http.Request, next http.Handler) *RequestData {
	URL := request.URL.RequestURI()
	p.coalesceLock.Lock()
	data, exists := p.coalesceMap[URL]
	if !exists {
		data = &RequestData{
			URL:     URL,
			Counter: 1,
			Next:    next,
			Request: request,
			Results: make(chan *ResponseData),
		}
		p.coalesceMap[URL] = data
	} else {
		data.Counter++
	}
	p.coalesceLock.Unlock()
	return data
}

func (p *coalescingProcessor) Start() {
	log.Printf("starting coalescing processor: interval=%v", p.interval)
	p.coalesceMap = make(map[string]*RequestData)
	go process(p)
}

func process(p *coalescingProcessor) {
	for {
		time.Sleep(p.interval)

		log.Println("Processing coalesce queue")
		p.coalesceLock.Lock()
		for _, data := range p.coalesceMap {
			go sendCoalesceRequest(data)
		}
		p.coalesceLock.Unlock()
	}
}

func sendCoalesceRequest(data *RequestData) {
	c := data.Counter
	if c == 0 {
		return
	}
	log.Printf("Coalescing %d requests for URL: %s\n\n", data.Counter, data.URL)
	rec := &responseRecorder{statusCodeChannel: make(chan int), bodyChannel: make(chan []byte)}
	go data.Next.ServeHTTP(rec, data.Request)

	statusCode := <-rec.statusCodeChannel
	body := <-rec.bodyChannel
	for data.Counter > 0 {
		log.Println(data.Counter)
		data.Results <- &ResponseData{StatusCode: statusCode, Body: body}
		data.Counter--
	}
}
