package cwebp

import "errors"

func cwebpError(msg string) error {
	return errors.New("cwebp: " + msg)
}

var (
	ErrRequiresCgo = cwebpError("requires cgo with the libwebp libraries")

	ErrConfigInitFailed     = cwebpError("failed to init config")
	ErrInvalidConfigOptions = cwebpError("invalid config options")

	ErrPictureInitFailed   = cwebpError("failed to init picture")
	ErrPictureImportFailed = cwebpError("failed to import picture from Go")

	ErrEncodeFailed = cwebpError("encode failed")
	ErrDecodeFailed = cwebpError("webp decode failed")

	ErrAnimationInvalid                  = cwebpError("invalid animation configuration")
	ErrAnimationInvalidFrameSize         = cwebpError("invalid frame size")
	ErrAnimationOptionsInitFailed        = cwebpError("failed to init animation options")
	ErrAnimationEncoderInitFailed        = cwebpError("failed to create animation encoder")
	ErrAnimationFrameBoundsInvalid       = cwebpError("animation frame bounds incorrect")
	ErrAnimationEncoderAddFailed         = cwebpError("failed to add frame to encoder")
	ErrAnimationEncoderFinalizeFailed    = cwebpError("failed to finalize frame data")
	ErrAnimationEncoderAssemblyFailed    = cwebpError("failed to assemble animated webp")
	ErrAnimationDecoderOptionsInitFailed = cwebpError("failed to init animation decoder")
)
