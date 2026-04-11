package vault

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

var Magic = [8]byte{'F', 'O', 'R', 'G', 'E', 'D', 0x00, 0x01}

const (
	CurrentVersion   uint16 = 2
	ProtectedKeySize        = 60 // nonce(12) + ciphertext(32) + tag(16)
	HeaderSize              = 8 + 2 + SaltSize + 4 + 4 + 1 + ProtectedKeySize + NonceSize // 123 bytes
)

type Header struct {
	Version      uint16
	KDF          KDFParams
	ProtectedKey [ProtectedKeySize]byte
	Nonce        [NonceSize]byte
}

func WriteHeader(w io.Writer, h Header) error {
	if _, err := w.Write(Magic[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h.Version); err != nil {
		return err
	}
	if _, err := w.Write(h.KDF.Salt[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h.KDF.TimeCost); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h.KDF.MemoryCost); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h.KDF.Parallelism); err != nil {
		return err
	}
	if _, err := w.Write(h.ProtectedKey[:]); err != nil {
		return err
	}
	if _, err := w.Write(h.Nonce[:]); err != nil {
		return err
	}
	return nil
}

func ReadHeader(r io.Reader) (Header, error) {
	var magic [8]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return Header{}, fmt.Errorf("reading magic: %w", err)
	}
	if magic != Magic {
		return Header{}, fmt.Errorf("not a forged vault file (invalid magic bytes)")
	}

	var h Header
	if err := binary.Read(r, binary.LittleEndian, &h.Version); err != nil {
		return Header{}, fmt.Errorf("reading version: %w", err)
	}
	if h.Version != CurrentVersion {
		return Header{}, fmt.Errorf("vault version %d is not supported (expected %d), please recreate your vault", h.Version, CurrentVersion)
	}

	if _, err := io.ReadFull(r, h.KDF.Salt[:]); err != nil {
		return Header{}, fmt.Errorf("reading salt: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.KDF.TimeCost); err != nil {
		return Header{}, fmt.Errorf("reading time cost: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.KDF.MemoryCost); err != nil {
		return Header{}, fmt.Errorf("reading memory cost: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &h.KDF.Parallelism); err != nil {
		return Header{}, fmt.Errorf("reading parallelism: %w", err)
	}
	if _, err := io.ReadFull(r, h.ProtectedKey[:]); err != nil {
		return Header{}, fmt.Errorf("reading protected key: %w", err)
	}
	if _, err := io.ReadFull(r, h.Nonce[:]); err != nil {
		return Header{}, fmt.Errorf("reading nonce: %w", err)
	}

	return h, nil
}

func MarshalVault(h Header, ciphertext []byte) []byte {
	var buf bytes.Buffer
	buf.Grow(HeaderSize + len(ciphertext))
	WriteHeader(&buf, h)
	buf.Write(ciphertext)
	return buf.Bytes()
}

func UnmarshalVault(data []byte) (Header, []byte, error) {
	if len(data) < HeaderSize {
		return Header{}, nil, fmt.Errorf("vault file too small (%d bytes)", len(data))
	}
	r := bytes.NewReader(data)
	h, err := ReadHeader(r)
	if err != nil {
		return Header{}, nil, err
	}
	ciphertext := data[HeaderSize:]
	return h, ciphertext, nil
}
