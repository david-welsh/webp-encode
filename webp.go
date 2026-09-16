//go:build cgo

package cwebp

/*
#cgo pkg-config: libwebp libwebpmux libwebpdemux
#include <stdlib.h>
#include <webp/encode.h>
#include <webp/mux.h>
#include <webp/decode.h>
#include <webp/demux.h>
*/
import "C"

import (
	"fmt"
	"image"
	"image/draw"
	"io"
	"runtime"
	"unsafe"
)

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

func Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty input", ErrDecodeFailed)
	}

	var width, height C.int

	buf := C.WebPDecodeRGBA(
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		&width,
		&height,
	)
	if buf == nil {
		return nil, fmt.Errorf("%w", ErrDecodeFailed)
	}
	defer C.WebPFree(unsafe.Pointer(buf))

	w := int(width)
	h := int(height)
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%w: invalid dimensions (%d,%d)", ErrDecodeFailed, w, h)
	}

	size := w * h * 4
	pix := C.GoBytes(unsafe.Pointer(buf), C.int(size))

	return &image.RGBA{
		Pix:    pix,
		Stride: w * 4,
		Rect:   image.Rect(0, 0, w, h),
	}, nil
}

func DecodeAll(r io.Reader) (*Webp, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty input", ErrDecodeFailed)
	}

	var pinner runtime.Pinner
	pinner.Pin(&data[0])
	defer pinner.Unpin()

	webpData := C.WebPData{
		bytes: (*C.uint8_t)(unsafe.Pointer(&data[0])),
		size:  C.size_t(len(data)),
	}

	var opts C.WebPAnimDecoderOptions
	if C.WebPAnimDecoderOptionsInit(&opts) == 0 {
		return nil, ErrAnimationDecoderOptionsInitFailed
	}
	opts.color_mode = C.MODE_RGBA
	opts.use_threads = 1

	dec := C.WebPAnimDecoderNew(&webpData, &opts)
	if dec == nil {
		return nil, ErrDecodeFailed
	}
	defer C.WebPAnimDecoderDelete(dec)

	var info C.WebPAnimInfo
	if C.WebPAnimDecoderGetInfo(dec, &info) == 0 {
		return nil, fmt.Errorf("%w: failed to read animation info", ErrDecodeFailed)
	}

	width := int(info.canvas_width)
	height := int(info.canvas_height)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("%w: invalid canvas dimensions (%d,%d)", ErrDecodeFailed, width, height)
	}

	result := &Webp{
		Frames:    make([]image.Image, 0, int(info.frame_count)),
		Delays:    make([]int, 0, int(info.frame_count)),
		LoopCount: int(info.loop_count),
	}

	var previousTimestamp int
	for C.WebPAnimDecoderHasMoreFrames(dec) != 0 {
		var buf *C.uint8_t
		var timestamp C.int

		if C.WebPAnimDecoderGetNext(dec, &buf, &timestamp) == 0 {
			return nil, fmt.Errorf("%w: failed decoding frame %d", ErrDecodeFailed, len(result.Frames))
		}

		size := width * height * 4
		pix := C.GoBytes(unsafe.Pointer(buf), C.int(size))

		frame := &image.RGBA{
			Pix:    pix,
			Stride: width * 4,
			Rect:   image.Rect(0, 0, width, height),
		}

		ts := int(timestamp)
		result.Frames = append(result.Frames, frame)
		result.Delays = append(result.Delays, ts-previousTimestamp)
		previousTimestamp = ts
	}

	if len(result.Frames) == 0 {
		return nil, fmt.Errorf("%w: no frames", ErrDecodeFailed)
	}
	return result, nil
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
