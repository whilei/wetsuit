package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// EchoStream takes a stream and a prefix and continuously prints lines
// from the stream prefixed by prefix.
func EchoStream(stream io.Reader, prefix string) {
	var (
		reader = bufio.NewReader(stream)
		buffer bytes.Buffer
	)
	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return
		}

		buffer.Write(line)
		if isPrefix {
			continue
		}

		fmt.Println(prefix, buffer.String())
		buffer.Reset()
	}
}

