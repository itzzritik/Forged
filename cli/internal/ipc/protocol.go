package ipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"unicode"
)

const (
	maxRequestMessageBytes  = 100 << 20
	maxResponseMessageBytes = 64 << 20
)

type Request struct {
	Command string          `json:"command"`
	Args    json.RawMessage `json:"args,omitempty"`
}

type Response struct {
	Status   string          `json:"status"`
	Data     json.RawMessage `json:"data,omitempty"`
	Error    string          `json:"error,omitempty"`
	finalize func(bool) error
}

func (r *Response) finalizeDelivery(delivered bool) error {
	if r.finalize == nil {
		return nil
	}
	finalize := r.finalize
	r.finalize = nil
	return finalize(delivered)
}

func OkResponse(data any) Response {
	var raw json.RawMessage
	if data != nil {
		b, _ := json.Marshal(data)
		raw = b
	}
	return Response{Status: "ok", Data: raw}
}

func ErrorResponse(err error) Response {
	return Response{Status: "error", Error: sentenceCase(err.Error())}
}

func WriteMessage(conn net.Conn, v any, limit int) error {
	data, err := json.Marshal(v)
	if err != nil {
		clear(data)
		return fmt.Errorf("Marshaling message: %w", err)
	}
	defer clear(data)
	if len(data) > limit {
		return fmt.Errorf("Message too large (%d bytes; limit %d bytes)", len(data), limit)
	}

	length := uint32(len(data))
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		return fmt.Errorf("Writing length: %w", err)
	}
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("Writing payload: %w", err)
	}
	return nil
}

func ReadMessage(conn net.Conn, v any, limit uint32) error {
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("Reading length: %w", err)
	}

	if length > limit {
		return fmt.Errorf("Message too large (%d bytes; limit %d bytes)", length, limit)
	}

	buf := make([]byte, length)
	defer clear(buf)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("Reading payload: %w", err)
	}

	if err := json.Unmarshal(buf, v); err != nil {
		return fmt.Errorf("Unmarshaling message: %w", err)
	}
	return nil
}

func validateMessageSize(v any, limit int) error {
	data, err := json.Marshal(v)
	if err != nil {
		clear(data)
		return fmt.Errorf("Marshaling message: %w", err)
	}
	defer clear(data)
	if len(data) > limit {
		return fmt.Errorf("Message too large (%d bytes; limit %d bytes)", len(data), limit)
	}
	return nil
}

func sentenceCase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) == 0 || !unicode.IsLower(runes[0]) {
		return trimmed
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
