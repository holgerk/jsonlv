package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	webview "github.com/webview/webview_go"
)

//go:embed index.html
var htmlContent string

const maxHistory = 50000

// logMsg is the envelope sent over SSE.
type logMsg struct {
	S string `json:"s"` // source: basename of file, or "" for stdin
	D string `json:"d"` // data:   original log line
}

type broker struct {
	mu      sync.Mutex
	history []logMsg
	clients map[chan logMsg]struct{}
}

func newBroker() *broker {
	return &broker{clients: make(map[chan logMsg]struct{})}
}

func (b *broker) publish(source, line string) {
	msg := logMsg{S: source, D: line}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.history) >= maxHistory {
		b.history = b.history[1:]
	}
	b.history = append(b.history, msg)
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (b *broker) subscribe() ([]logMsg, chan logMsg) {
	b.mu.Lock()
	defer b.mu.Unlock()
	hist := make([]logMsg, len(b.history))
	copy(hist, b.history)
	ch := make(chan logMsg, 1024)
	b.clients[ch] = struct{}{}
	return hist, ch
}

func (b *broker) unsubscribe(ch chan logMsg) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

func sendMsg(w http.ResponseWriter, flusher http.Flusher, msg logMsg) {
	data, _ := json.Marshal(msg)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

var globalBroker *broker

var (
	tailedMu    sync.Mutex
	tailedFiles = map[string]bool{}
)

func startTailing(path string) {
	tailedMu.Lock()
	already := tailedFiles[path]
	if !already {
		tailedFiles[path] = true
	}
	tailedMu.Unlock()
	if already {
		return
	}
	source := filepath.Base(path)
	go func() {
		tail, err := lastNLines(path, 1000)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			return
		}
		for _, line := range tail {
			if line != "" {
				globalBroker.publish(source, line)
			}
		}
		followFile(path, source, globalBroker)
	}()
}

func restartApp() {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	syscall.Exec(exe, os.Args, os.Environ()) //nolint:errcheck
}

// stdinIsPiped reports whether stdin is a pipe (not a terminal).
func stdinIsPiped() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func serveHTML() string {
	prefsMu.Lock()
	theme := curPrefs.Theme
	prefsMu.Unlock()
	if theme == "light" || theme == "solarized" {
		return strings.Replace(htmlContent, "<body>", `<body class="`+theme+`">`, 1)
	}
	return htmlContent
}

func main() {
	initMappings()

	prefs := loadPrefs()
	prefsMu.Lock()
	curPrefs = prefs
	prefsMu.Unlock()

	follow := flag.Bool("f", false, "follow file(s) for new lines")
	lines := flag.Int("n", 1000, "number of lines from end of file")
	flag.Parse()
	files := flag.Args()

	b := newBroker()
	globalBroker = b

	piped := stdinIsPiped()

	if len(files) == 0 && piped {
		// Read from stdin
		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
			for scanner.Scan() {
				b.publish("", scanner.Text())
			}
		}()
	} else {
		for _, path := range files {
			path := path
			source := filepath.Base(path)
			go func() {
				tail, err := lastNLines(path, *lines)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
					return
				}
				for _, line := range tail {
					if line != "" {
						b.publish(source, line)
					}
				}
				if *follow {
					followFile(path, source, b)
				}
			}()
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, serveHTML())
	})

	mux.HandleFunc("/prefs", func(w http.ResponseWriter, r *http.Request) {
		prefsMu.Lock()
		p := curPrefs
		prefsMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p) //nolint:errcheck
	})

	mux.HandleFunc("/set-theme", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		theme := strings.TrimSpace(string(body))
		if theme == "dark" || theme == "light" || theme == "solarized" {
			setThemePref(theme)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		hist, ch := b.subscribe()
		defer b.unsubscribe(ch)

		for i, msg := range hist {
			sendMsg(w, flusher, msg)
			if i%200 == 0 {
				flusher.Flush()
			}
		}
		flusher.Flush()

		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case msg := <-ch:
				sendMsg(w, flusher, msg)
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
	setupAppIcon()

	mux.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		file := r.URL.Query().Get("file")
		line := r.URL.Query().Get("line")
		if file == "" {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		local, ok := resolveLocalPath(file)
		if !ok {
			json.NewEncoder(w).Encode(map[string]string{"status": "not_found", "remote": file}) //nolint:errcheck
			return
		}
		psURL := "phpstorm://open?file=" + url.QueryEscape(local)
		if line != "" {
			psURL += "&line=" + line
		}
		exec.Command("open", psURL).Start() //nolint:errcheck
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	})

	mux.HandleFunc("/pick-file", func(w http.ResponseWriter, r *http.Request) {
		remote := r.URL.Query().Get("remote")
		result := make(chan string, 1)
		wv.Dispatch(func() { result <- PickLocalFile() })
		local := <-result
		if local == "" {
			w.WriteHeader(499) // user cancelled
			return
		}
		if remote != "" {
			addMapping(remote, local)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "local": local}) //nolint:errcheck
	})

	recent := loadRecent()
	SetupFileMenu(recent)

	go func() {
		for action := range menuFileCh {
			switch action {
			case "open":
				result := make(chan []string, 1)
				wv.Dispatch(func() { result <- PickMultipleFiles() })
				paths := <-result
				if len(paths) == 0 {
					continue
				}
				for _, p := range paths {
					startTailing(p)
				}
				recent := addRecent(paths)
				wv.Dispatch(func() { RebuildRecentMenu(recent) })
			case "clear":
				clearRecent()
				wv.Dispatch(func() { RebuildRecentMenu(nil) })
			case "restart":
				restartApp()
			default:
				startTailing(action)
				recent := addRecent([]string{action})
				wv.Dispatch(func() { RebuildRecentMenu(recent) })
			}
		}
	}()

	wv.SetTitle("Log Viewer")

	// Restore saved window size, or use default.
	savedW, savedH := prefs.Window.W, prefs.Window.H
	if savedW < 640 || savedH < 400 {
		savedW, savedH = 1280, 800
	}
	wv.SetSize(int(savedW), int(savedH), webview.HintNone)

	wv.Bind("nativeQuit", func() { os.Exit(0) }) //nolint:errcheck
	wv.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port))

	// Restore saved window position (after Navigate so the window exists).
	if prefs.Window.X != 0 || prefs.Window.Y != 0 {
		wv.Dispatch(func() {
			SetWindowFrame(wv.Window(), prefs.Window.X, prefs.Window.Y, savedW, savedH)
		})
	}

	// Ask to reopen recent files when launched without arguments and not piped.
	if len(files) == 0 && !piped && len(recent) > 0 {
		go func() {
			time.Sleep(400 * time.Millisecond)
			result := make(chan bool, 1)
			wv.Dispatch(func() { result <- ShowReopenAlert(len(recent)) })
			if <-result {
				for _, p := range recent {
					startTailing(p)
				}
			}
		}()
	}

	// Poll window frame every 2 s and persist changes.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		var last [4]float64
		for range ticker.C {
			ch := make(chan [4]float64, 1)
			wv.Dispatch(func() {
				x, y, w, h := GetWindowFrame(wv.Window())
				ch <- [4]float64{x, y, w, h}
			})
			frame := <-ch
			if frame != last && frame[2] >= 640 && frame[3] >= 400 {
				last = frame
				setWindowPref(frame[0], frame[1], frame[2], frame[3])
			}
		}
	}()

	wv.Run()
}
