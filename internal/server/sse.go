package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	buf     bytes.Buffer
}

func NewSSEWriter(w http.ResponseWriter) *SSEWriter {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	return &SSEWriter{w: w, flusher: flusher}
}

func (s *SSEWriter) WriteEvent(event string, data interface{}) error {
	s.buf.Reset()
	fmt.Fprintf(&s.buf, "event: %s\ndata: ", event)
	if err := json.NewEncoder(&s.buf).Encode(data); err != nil {
		return err
	}
	s.buf.WriteByte('\n')
	if _, err := s.w.Write(s.buf.Bytes()); err != nil {
		return err
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

func (s *SSEWriter) Flush() {
	if s.flusher != nil {
		s.flusher.Flush()
	}
}
