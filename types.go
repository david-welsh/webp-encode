package cwebp

import "image"

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
