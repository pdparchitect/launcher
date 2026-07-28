package runtime

import (
	"io"
	"strings"
	"sync"
)

type progressWriter struct {
	destination io.Writer
	report      func(string)
	buffer      string
	mu          sync.Mutex
}

func newProgressWriter(
	destination io.Writer,
	report func(string),
) *progressWriter {
	if destination == nil {
		destination = io.Discard
	}
	return &progressWriter{destination: destination, report: report}
}

func (writer *progressWriter) Write(data []byte) (int, error) {
	written, err := writer.destination.Write(data)
	if written > 0 {
		writer.accept(string(data[:written]))
	}
	return written, err
}

func (writer *progressWriter) Flush() {
	writer.mu.Lock()
	line := strings.TrimSpace(writer.buffer)
	writer.buffer = ""
	writer.mu.Unlock()
	writer.emit(line)
}

func (writer *progressWriter) accept(value string) {
	writer.mu.Lock()
	value = writer.buffer + strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	writer.buffer = lines[len(lines)-1]
	complete := append([]string(nil), lines[:len(lines)-1]...)
	writer.mu.Unlock()

	for _, line := range complete {
		writer.emit(strings.TrimSpace(line))
	}
}

func (writer *progressWriter) emit(line string) {
	if line != "" && writer.report != nil {
		writer.report(line)
	}
}
