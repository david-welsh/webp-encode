package cwebp

import (
	"bytes"
	_ "embed"
	"image/jpeg"
	"testing"
)

//go:embed testdata/anim.webp
var animWebpBytes []byte

//go:embed testdata/still.jpg
var stillJPGBytes []byte

func TestDecodeAll(t *testing.T) {
	got, err := DecodeAll(bytes.NewBuffer(animWebpBytes))
	if err != nil {
		t.Fatalf("failed to decode animation: %s", err.Error())
	}

	if len(got.Frames) != 19 {
		t.Fatalf("expected 19 frames, got %d", len(got.Frames))
	}

	buf := bytes.NewBuffer([]byte{})
	err = EncodeAll(buf, got, nil)
	if err != nil {
		t.Fatalf("failed to encode animation: %s", err.Error())
	}
}

func TestDecode(t *testing.T) {
	got, err := jpeg.Decode(bytes.NewBuffer(stillJPGBytes))
	if err != nil {
		t.Fatalf("failed to decode jpeg: %s", err.Error())
	}
	buf := bytes.NewBuffer([]byte{})
	err = Encode(buf, got, nil)
	if err != nil {
		t.Fatalf("failed to encode webp: %s", err.Error())
	}

	_, err = Decode(buf)
	if err != nil {
		t.Fatalf("failed to decode webp: %s", err.Error())
	}
}
