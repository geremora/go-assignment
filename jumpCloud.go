package main

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const ERROR_ID_LOWER_ZERO string = "Error - id must be greater than 0"
const ERROR_PASSWORD_EMPTY string = "password is empty or field not present"
const ERROR_ID_FORMAT string = "invalid id format"
const ERROR_INVALID_FORM string = "invalid form"
const fakeLatency = 5

type server struct {
	instance      *http.Server
	w             http.ResponseWriter
	r             *http.Request
	hashes        []string
	passwordMap   map[string]int
	shutdownDone  chan struct{}
	m             sync.Mutex
	doingHash     *sync.Cond
	totalRequests int64
	totalTime     time.Duration
}

type stats struct {
	Total   int64
	Average time.Duration
}

func main() {
	hs := server{}

	hs.instance = &http.Server{}
	hs.instance.Addr = ":9090"
	hs.shutdownDone = make(chan struct{})
	hs.passwordMap = make(map[string]int)
	hs.hashes = make([]string, 0)
	hs.doingHash = sync.NewCond(&hs.m)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hs.r = r
		hs.w = w
		hs.routing()
	})

	var err error
	if err = hs.instance.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			err = nil
			// let shutdown() know the server is shut down
			<-hs.shutdownDone
		}
	}
}

func (hs *server) generateHash() {

	defer func(start time.Time) {
		hs.m.Lock()
		hs.totalTime += time.Since(start)
		hs.totalRequests++
		hs.m.Unlock()
	}(time.Now())

	err := hs.r.ParseForm()
	if err != nil {
		fmt.Println(ERROR_INVALID_FORM)
		http.Error(hs.w, err.Error(), http.StatusInternalServerError)
		return
	}

	password := hs.r.PostFormValue("password")

	if len(password) == 0 {
		fmt.Println(ERROR_PASSWORD_EMPTY)
		http.Error(hs.w, ERROR_PASSWORD_EMPTY, http.StatusInternalServerError)
		return
	}

	hs.m.Lock()

	id := hs.passwordMap[password]

	if id == 0 {

		id = len(hs.hashes) + 1
		hs.passwordMap[password] = id
		hs.hashes = append(hs.hashes, "")
		go hs.startHash(id, password)

	}
	hs.m.Unlock()
	fmt.Fprintf(hs.w, "id generated: %d\n", id)

}

func (hs *server) showStats() {

	s := stats{hs.totalRequests, (hs.totalTime / time.Millisecond / time.Duration(hs.totalRequests))}

	js, err := json.Marshal(s)
	if err != nil {
		http.Error(hs.w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(hs.w, "%s", js)
}

func (hs *server) lookupHash() {

	id, err := strconv.Atoi(hs.r.URL.Path[len("/hash/"):])

	if err != nil {
		fmt.Println(ERROR_ID_FORMAT, err)
		http.Error(hs.w, ERROR_ID_FORMAT, http.StatusNotFound)
		return
	}

	if id <= 0 {
		fmt.Println(ERROR_ID_LOWER_ZERO)
		fmt.Fprintf(hs.w, ERROR_ID_LOWER_ZERO)
		return
	}

	hs.m.Lock()

	for hs.hashes[id-1] == "" {
		hs.doingHash.Wait()
	}

	hs.m.Unlock()
	h := hs.hashes[id-1]
	fmt.Fprintf(hs.w, "%s", h)
}

func (hs *server) routing() {

	switch {
	case strings.Index(hs.r.URL.Path, "/hash/") == 0:
		hs.lookupHash()
	case hs.r.URL.Path == "/hash":
		hs.generateHash()
	case hs.r.URL.Path == "/shutdown":
		hs.shutdown()
	case hs.r.URL.Path == "/stats":
		hs.showStats()
	}
}

func (hs *server) startHash(id int, password string) {
	defer func(start time.Time) {
		hs.m.Lock()
		hs.totalTime += time.Since(start)
		hs.m.Unlock()
	}(time.Now())

	time.Sleep(fakeLatency * time.Second)

	h := SHA512(password)
	hs.m.Lock()
	hs.hashes[id-1] = h
	hs.m.Unlock()

	hs.doingHash.Broadcast()
}

func SHA512(text string) string {
	algorithm := sha512.New()
	algorithm.Write([]byte(text))
	return base64.StdEncoding.EncodeToString(algorithm.Sum(nil))
}

func (hs *server) shutdown() {

	go func() {
		ctx, _ := context.WithTimeout(context.Background(), (fakeLatency*2)*time.Second)
		if err := hs.instance.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
		hs.shutdownDone <- struct{}{}
	}()
}
