package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	webview "github.com/webview/webview_go"
)

//go:embed index.html
var htmlContent string

const maxHistory = 50000

type broker struct {
	mu      sync.Mutex
	history []string
	clients map[chan string]struct{}
}

func newBroker() *broker {
	return &broker{clients: make(map[chan string]struct{})}
}

func (b *broker) publish(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.history) >= maxHistory {
		b.history = b.history[1:]
	}
	b.history = append(b.history, line)
	for ch := range b.clients {
		select {
		case ch <- line:
		default:
		}
	}
}

func (b *broker) subscribe() ([]string, chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	hist := make([]string, len(b.history))
	copy(hist, b.history)
	ch := make(chan string, 1024)
	b.clients[ch] = struct{}{}
	return hist, ch
}

func (b *broker) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

func main() {
	b := newBroker()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			b.publish(scanner.Text())
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlContent)
	})

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		hist, ch := b.subscribe()
		defer b.unsubscribe(ch)

		for i, line := range hist {
			fmt.Fprintf(w, "data: %s\n\n", line)
			if i%200 == 0 {
				flusher.Flush()
			}
		}
		flusher.Flush()

		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case line := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			case <-heartbeat.C:
				fmt.Fprintf(w, ": keep-alive\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	go http.Serve(ln, mux) //nolint:errcheck

	wv := webview.New(true)
	defer wv.Destroy()
	setupAppMenu()
	wv.SetTitle("Log Viewer")
	wv.SetSize(1280, 800, webview.HintNone)
	wv.Bind("nativeQuit", func() { os.Exit(0) }) //nolint:errcheck
	wv.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port))
	wv.Run()
}
