package main

import (
	_ "embed"

	webview "github.com/webview/webview_go"
)

//go:embed index.html
var html string

func main() {
	w := webview.New(true)
	defer w.Destroy()
	w.SetTitle("Go WebView Demo")
	w.SetSize(1024, 768, webview.HintNone)
	w.SetHtml(html)
	w.Run()
}
