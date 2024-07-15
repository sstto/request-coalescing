package middleware

import (
	"log"
	"net/http"
	"sees/coalescing/core"
)

func RequestCoalescing(processor core.CoalescingProcessor, next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requestData := processor.AddRequest(r, next)
			resp := <-requestData.Results
			w.WriteHeader(resp.StatusCode)
			_, err := w.Write(resp.Body)
			if err != nil {
				log.Fatal(err)
				return
			}
		},
	)
}
