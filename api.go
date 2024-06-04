package main

import (
	"context"
	"io"
	"log"
	"net/http"
)

type apiBuffer struct {
	requests chan<- apiRequest
}

type apiRequest struct {
	verb     string
	key      string
	value    []byte
	response chan<- []byte
}

func startApi(ctx context.Context) <-chan struct{} {
	requests := make(chan apiRequest, 10)
	buffer := apiBuffer{requests: requests}

	done := make(chan struct{})
	// middleware
	// authentication
	srv := &http.Server{Addr: "0.0.0.0:8080"}
	http.HandleFunc("/", buffer.handle())
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			panic("ListenAndServe: " + err.Error())
		}
	}()
	loopDone := processingLoop(requests, ctx)
	go func() {
		defer close(done)

		<-ctx.Done()
		if err := srv.Shutdown(ctx); err != nil {
			log.Println("Shutdown: " + err.Error())
		}
		<-loopDone
	}()

	return done
}

func processingLoop(requests chan apiRequest, ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		go func() {
			<-ctx.Done()
			close(requests)
		}()
		data := make(map[string][]byte)
		for req := range requests {
			switch req.verb {
			case http.MethodDelete:
				delete(data, req.key)

			case http.MethodPut:
			case http.MethodPost:
			case http.MethodPatch:
				data[req.key] = req.value

			case http.MethodGet:
				value, ok := data[req.key]
				if !ok {
					log.Printf("could not find value for key %v", req.key)
				} else {
					req.response <- value
				}
			}
			close(req.response)
		}
	}()
	return done
}

func (buffer *apiBuffer) handle() func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		var err error
		defer func() {
			r := recover()
			if r != nil {
				log.Println(err)
				log.Println(r)
				rw.WriteHeader(http.StatusInternalServerError)
			}
		}()

		responseChan := make(chan []byte)
		request := apiRequest{verb: r.Method, key: r.URL.Path, response: responseChan}
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			body, err := io.ReadAll(io.Reader(r.Body))
			defer r.Body.Close()
			if err != nil {
				log.Println(err)
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			request.value = body
		}

		select {
		case buffer.requests <- request:
		default:
			log.Println("request buffer full or shutdown")
			rw.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		response, ok := <-responseChan

		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusOK)
			return
		}

		if ok {
			rw.WriteHeader(http.StatusOK)
			rw.Write(response)
		} else {
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}
}
