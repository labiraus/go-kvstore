package main

import (
	"context"
	"io"
	"log"
	"net/http"
)

var requestBuffer chan<- apiRequest

type apiRequest struct {
	verb     string
	key      string
	value    []byte
	response chan<- []byte
}

func startApi(ctx context.Context) <-chan struct{} {
	requests := make(chan apiRequest, 10)
	requestBuffer = requests
	done := make(chan struct{})
	// middleware
	// authentication
	srv := &http.Server{Addr: "0.0.0.0:8080"}
	http.HandleFunc("/", handle)
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			panic("ListenAndServe: " + err.Error())
		}
	}()
	loopDone := actor(requests, ctx)
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

func actor(requests chan apiRequest, ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	// Shutdown
	go func() {
		<-ctx.Done()
		close(requests)
	}()

	go func() {
		defer close(done)
		processLoop(requests)
	}()

	return done
}

func processLoop(requests <-chan apiRequest) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		store := make(map[string][]byte)
		defer close(done)
		for req := range requests {
			switch req.verb {
			case http.MethodDelete:
				delete(store, req.key)

			case http.MethodPut:
			case http.MethodPost:
			case http.MethodPatch:
				store[req.key] = req.value

			case http.MethodGet:
				value, ok := store[req.key]
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

func handle(rw http.ResponseWriter, r *http.Request) {
	defer func() {
		r := recover()
		if r != nil {
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
	case requestBuffer <- request:
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
