package ipc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

type Request struct {
	Command string          `json:"command"`
	Args    json.RawMessage `json:"args,omitempty"`
}

type Response struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  string          `json:"error,omitempty"`
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
	return Response{Status: "error", Error: err.Error()}
}

func WriteMessage(conn net.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshaling message: %w", err)
	}

	length := uint32(len(data))
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		return fmt.Errorf("writing length: %w", err)
	}
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("writing payload: %w", err)
	}
	return nil
}

func ReadMessage(conn net.Conn, v any) error {
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("reading length: %w", err)
	}

	if length > 10*1024*1024 {
		return fmt.Errorf("message too large (%d bytes)", length)
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("reading payload: %w", err)
	}

	if err := json.Unmarshal(buf, v); err != nil {
		return fmt.Errorf("unmarshaling message: %w", err)
	}
	return nil
}
