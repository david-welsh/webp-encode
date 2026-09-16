//go:build !cgo

package cwebp

func Encode(w io.Writer, m image.Image, o *Options) error {
	return ErrRequiresCgo
}

func EncodeAll(w io.Writer, a *Webp, o *Options) error {
	return ErrRequiresCgo
}

func Decode(r io.Reader) (image.Image, error) {
	return nil, ErrRequiresCgo
}

func DecodeAll(r io.Reader) (*Webp, error) {
	return nil, ErrRequiresCgo
}
