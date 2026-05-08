package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	webview "github.com/webview/webview_go"
)

//go:embed index.html
var html string

func main() {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle("Log Viewer")
	w.SetSize(1280, 800, webview.HintNone)
	w.SetHtml(html)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			escaped, _ := json.Marshal(line)
			js := fmt.Sprintf("addLog(%s)", string(escaped))
			w.Dispatch(func() {
				w.Eval(js)
			})
		}
	}()

	w.Run()
}
