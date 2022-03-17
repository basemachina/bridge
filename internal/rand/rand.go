package rand

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	mathrand "math/rand"
	"time"
)

// Read reads random data to specified p
func Read(p []byte) {
	_, err := io.ReadFull(rand.Reader, p)
	if err == nil {
		return
	}
	src := mathrand.NewSource(time.Now().UnixNano())
	reader := mathrand.New(src)
	// always return err == nil
	// https://pkg.go.dev/math/rand#Rand.Read
	io.ReadFull(reader, p)
}

// String returns random string
func String() string {
	p := make([]byte, 16)
	Read(p)
	return base64.StdEncoding.EncodeToString(p)
}
