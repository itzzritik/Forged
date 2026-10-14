package ipc

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

// feed writes raw bytes into one end of a pipe and returns the other end for ReadMessage.
func feed(t *testing.T, raw []byte) net.Conn {
	t.Helper()
	client, server := net.Pipe()
	go func() {
		_, _ = client.Write(raw)
		_ = client.Close()
	}()
	t.Cleanup(func() { _ = server.Close() })
	_ = server.SetDeadline(time.Now().Add(5 * time.Second))
	return server
}

func frame(length uint32, payload string) []byte {
	return append(binary.BigEndian.AppendUint32(nil, length), payload...)
}

func TestMessageRoundTrip(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	sent := Request{Command: "status", Args: []byte(`{"name":"work"}`)}
	go func() { _ = WriteMessage(client, sent, 1<<20) }()
	var got Request
	if err := ReadMessage(server, &got, 1<<20); err != nil {
		t.Fatal(err)
	}
	if got.Command != sent.Command || string(got.Args) != string(sent.Args) {
		t.Fatalf("got %+v, want %+v", got, sent)
	}
}

func TestReadMessageRejectsBadFrames(t *testing.T) {
	cases := map[string]struct {
		raw  []byte
		want string
	}{
		"length over limit": {frame(1<<31, ""), "too large"},
		"truncated payload": {frame(50, `{"command":`), "Reading payload"},
		"truncated length":  {[]byte{0, 0}, "Reading length"},
		"bad json":          {frame(3, "{{{"), "Unmarshaling"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var req Request
			err := ReadMessage(feed(t, tc.raw), &req, 1<<20)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestWriteMessageRefusesOversized(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	if err := WriteMessage(client, strings.Repeat("x", 100), 10); err == nil {
		t.Fatal("oversized message was written")
	}
}

func FuzzReadMessage(f *testing.F) {
	f.Add(frame(2, "{}"))
	f.Add(frame(16, `{"command":"x"}`))
	f.Add(frame(0xFFFFFFFF, "{"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		var req Request
		_ = ReadMessage(feed(t, raw), &req, 4096)
	})
}
