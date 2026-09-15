package cwebp

/*
#cgo pkg-config: libwebp libwebpmux
#include <stdlib.h>
#include <webp/encode.h>
#include <webp/mux.h>
*/
import "C"

import (
	"fmt"
	"image"
	"image/draw"
	"io"
	"unsafe"
)

type Options struct {
	Lossless bool
	Quality  float32
	Method   int
}

type Webp struct {
	Frames    []image.Image
	Delays    []int
	LoopCount int
}

func Encode(w io.Writer, m image.Image, o *Options) error {
	cfg, err := buildConfig(o)
	if err != nil {
		return err
	}

	rgba := toRGBA(m)

	var pic C.WebPPicture
	err = newPicture(rgba, &pic)
	defer C.WebPPictureFree(&pic)
	if err != nil {
		return err
	}

	writer := (*C.WebPMemoryWriter)(C.malloc(C.sizeof_WebPMemoryWriter))
	defer C.free(unsafe.Pointer(writer))
	C.WebPMemoryWriterInit(writer)
	defer C.WebPMemoryWriterClear(writer)
	pic.writer = C.WebPWriterFunction(C.WebPMemoryWrite)
	pic.custom_ptr = unsafe.Pointer(writer)

	if C.WebPEncode(&cfg, &pic) == 0 {
		return fmt.Errorf("%w: error code %d", ErrEncodeFailed, int(pic.error_code))
	}

	_, err = w.Write(C.GoBytes(unsafe.Pointer(writer.mem), C.int(writer.size)))
	return err
}

func EncodeAll(w io.Writer, a *Webp, o *Options) error {
	if a == nil || len(a.Frames) == 0 || len(a.Delays) != len(a.Frames) {
		return ErrAnimationInvalid
	}

	b := a.Frames[0].Bounds()
	width, height := b.Dx(), b.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: (%d,%d)", ErrAnimationInvalidFrameSize, width, height)
	}

	var animOpts C.WebPAnimEncoderOptions
	if C.WebPAnimEncoderOptionsInit(&animOpts) == 0 {
		return ErrAnimationOptionsInitFailed
	}
	animOpts.anim_params.loop_count = C.int(a.LoopCount)

	enc := C.WebPAnimEncoderNew(C.int(width), C.int(height), &animOpts)
	if enc == nil {
		return ErrAnimationEncoderInitFailed
	}
	defer C.WebPAnimEncoderDelete(enc)

	cfg, err := buildConfig(o)
	if err != nil {
		return err
	}

	timestampMS := 0
	for i, img := range a.Frames {
		err = encodeFrame(enc, cfg, img, timestampMS, i, width, height)
		if err != nil {
			return err
		}
		timestampMS += a.Delays[i]
	}

	if C.WebPAnimEncoderAdd(enc, nil, C.int(timestampMS), nil) == 0 {
		return fmt.Errorf("%w: %s", ErrAnimationEncoderFinalizeFailed, C.GoString(C.WebPAnimEncoderGetError(enc)))
	}

	var data C.WebPData
	if C.WebPAnimEncoderAssemble(enc, &data) == 0 {
		return fmt.Errorf("%w: %s", ErrAnimationEncoderAssemblyFailed, C.GoString(C.WebPAnimEncoderGetError(enc)))
	}
	defer C.WebPDataClear(&data)

	_, err = w.Write(C.GoBytes(unsafe.Pointer(data.bytes), C.int(data.size)))
	return err
}

func encodeFrame(
	enc *C.WebPAnimEncoder,
	cfg C.WebPConfig,
	img image.Image,
	frameTimeStamp int,
	frameNumber int,
	width, height int,
) error {
	frame := toRGBA(img)
	frameBounds := frame.Bounds()
	if frameBounds.Dx() != width || frameBounds.Dy() != height {
		return fmt.Errorf(
			"%w: frame(%d:%d,%d), canvas(%d,%d)",
			ErrAnimationFrameBoundsInvalid,
			frameNumber,
			frameBounds.Dx(),
			frameBounds.Dy(),
			width,
			height,
		)
	}

	var pic C.WebPPicture
	err := newPicture(frame, &pic)
	defer C.WebPPictureFree(&pic)
	if err != nil {
		return err
	}

	if C.WebPAnimEncoderAdd(enc, &pic, C.int(frameTimeStamp), &cfg) == 0 {
		return fmt.Errorf("%w: %s", ErrAnimationEncoderAddFailed, C.GoString(C.WebPAnimEncoderGetError(enc)))
	}
	return nil
}

func buildConfig(o *Options) (C.WebPConfig, error) {
	opt := Options{
		Quality: 75,
	}
	if o != nil {
		opt = *o
	}

	var cfg C.WebPConfig
	if C.WebPConfigInit(&cfg) == 0 {
		return cfg, ErrConfigInitFailed
	}

	cfg.quality = C.float(opt.Quality)

	if opt.Method > 0 {
		cfg.method = C.int(opt.Method)
	}

	if opt.Lossless {
		cfg.lossless = 1
	}

	if C.WebPValidateConfig(&cfg) == 0 {
		return cfg, ErrInvalidConfigOptions
	}

	return cfg, nil
}

func toRGBA(m image.Image) *image.RGBA {
	if rgba, ok := m.(*image.RGBA); ok && rgba.Rect.Min == (image.Point{}) {
		return rgba
	}

	b := m.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), m, b.Min, draw.Src)
	return rgba
}

func newPicture(img *image.RGBA, pic *C.WebPPicture) error {
	if C.WebPPictureInit(pic) == 0 {
		return ErrPictureInitFailed
	}

	pic.width = C.int(img.Bounds().Dx())
	pic.height = C.int(img.Bounds().Dy())
	pic.use_argb = 1

	if len(img.Pix) == 0 {
		return ErrPictureInitFailed
	}

	res := C.WebPPictureImportRGBA(pic, (*C.uint8_t)(unsafe.Pointer(&img.Pix[0])), C.int(img.Stride))
	if res == 0 {
		return ErrPictureImportFailed
	}

	return nil
}
