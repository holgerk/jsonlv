package main

// This file only contains //export functions.
// CGO rule: files with //export must not have C definitions in their preamble.

/*
#include <CoreGraphics/CoreGraphics.h>
*/
import "C"

//export cMenuOpenFiles
func cMenuOpenFiles() {
	select {
	case menuFileCh <- "open":
	default:
	}
}

//export cOpenFile
func cOpenFile(path *C.char) {
	menuFileCh <- C.GoString(path)
}

//export cClearRecent
func cClearRecent() {
	select {
	case menuFileCh <- "clear":
	default:
	}
}

//export cRestartApp
func cRestartApp() {
	select {
	case menuFileCh <- "restart":
	default:
	}
}

//export cClearLogFiles
func cClearLogFiles() {
	select {
	case menuFileCh <- "clear-log-files":
	default:
	}
}

//export cSaveWindowFrame
func cSaveWindowFrame(x, y, w, h C.CGFloat) {
	setWindowPref(float64(x), float64(y), float64(w), float64(h))
}
