package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"time"
)

// lastNLines returns the last n non-empty lines of a file by reading backwards.
func lastNLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	if size == 0 || n == 0 {
		return nil, nil
	}

	const chunk = 32 * 1024
	buf := make([]byte, 0, chunk)
	pos := size

	for {
		read := int64(chunk)
		if pos < read {
			read = pos
		}
		pos -= read
		tmp := make([]byte, read)
		if _, err := f.ReadAt(tmp, pos); err != nil {
			return nil, err
		}
		buf = append(tmp, buf...)
		if bytes.Count(buf, []byte{'\n'}) > n || pos == 0 {
			break
		}
	}

	lines := strings.Split(strings.TrimRight(string(buf), "\r\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

// followFile watches path for new appended lines and publishes them.
func followFile(path, source string, b *broker) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	if _, err = f.Seek(0, io.SeekEnd); err != nil {
		return
	}

	var partial []byte
	buf := make([]byte, 64*1024)

	for {
		n, _ := f.Read(buf)
		if n > 0 {
			data := append(partial, buf[:n]...)
			for {
				i := bytes.IndexByte(data, '\n')
				if i < 0 {
					break
				}
				line := strings.TrimRight(string(data[:i]), "\r")
				data = data[i+1:]
				if line != "" {
					b.publish(source, line)
				}
			}
			partial = append(partial[:0], data...)
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
}
