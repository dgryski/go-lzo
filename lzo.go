// Package lzo provides access to the LZO library, version 2.05
/*

License: GPLv3 or later

Copyright (C) 2011 Damian Gryski <damian@gryski.com>
*/
package lzo

/*
#cgo LDFLAGS: -llzo2 -L/opt/local/lib/

#include <lzo/lzoconf.h>
#include <lzo/lzo1x.h>

// lzo_init is a macro -- we need a function so we can call it from Go
static int my_lzo_init(void) { return lzo_init(); }

// how big a work buffer do we need to allocate for this algorithm
// again, a macro so we need to be able to call it from Go
static int lzo1x_1_mem_compress() { return LZO1X_1_MEM_COMPRESS; }
static int lzo1x_999_mem_compress() { return LZO1X_999_MEM_COMPRESS; }

*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Errno is an lzo error
type Errno int

var errText = map[Errno]string{
	0:  "ok",
	-1: "error",
	-2: "[unused] out of memory",
	-3: "[unused] not compressible",
	-4: "input overrun",
	-5: "output overrun",
	-6: "lookbehind overrun",
	-7: "eof not found",
	-8: "input not consumed",
	-9: "[unused] not yet implemented",
}

func (e Errno) Error() string {

	s := errText[e]
	if s == "" {
		return fmt.Sprintf("errno %d", int(e))
	}
	return s
}

var (
	ErrOk                = Errno(0)
	ErrError             = Errno(-1)
	ErrOutOfMemory       = Errno(-2) /* [not used right now] */
	ErrNotCompressible   = Errno(-3) /* [not used right now] */
	ErrInputOverrun      = Errno(-4)
	ErrOutputOverrun     = Errno(-5)
	ErrLookbehindOverrun = Errno(-6)
	ErrEofNotFound       = Errno(-7)
	ErrInputNotConsumed  = Errno(-8)
	ErrNotYetImplemented = Errno(-9) /* [not used right now] */
)

type LzoAlgorithm int

const (
	Lzo1x_1 LzoAlgorithm = iota
	Lzo1x_999
	BestSpeed          = Lzo1x_1
	BestCompression    = Lzo1x_999
	DefaultCompression = Lzo1x_999
)

type Compressor struct {
	level      LzoAlgorithm
	compress   func([]byte, []byte, *int, []byte) C.int
	wrkmem_len int
}

func init() {
	if err := C.my_lzo_init(); err != 0 {
		panic("lzo library initialization failed")
	}
}

func NewCompressor(level LzoAlgorithm) (*Compressor, error) {

	z := new(Compressor)
	z.level = level

	switch z.level {
	case Lzo1x_1:
		z.compress = lzo1x_1_compress
		z.wrkmem_len = int(C.lzo1x_1_mem_compress())
	case Lzo1x_999:
		z.compress = lzo1x_999_compress
		z.wrkmem_len = int(C.lzo1x_999_mem_compress())
	}

	return z, nil
}

// Version returns the version of the LZO library being used
func Version() string {
	p := C.lzo_version_string()
	return C.GoString(p)
}

// Compress compresses a byte array and returns the compressed stream
func (z *Compressor) Compress(b []byte) ([]byte, error) {

	// our output buffer, sized to contain a worst-case compression
	out_size := lzo1x_1_output_size(len(b))
	out := make([]byte, out_size)

	out_size = 0 // here it's used to store the size of the compressed data

	var err C.int
	wrkmem := make([]byte, z.wrkmem_len)
	err = z.compress(b, out, &out_size, wrkmem)

	// compression failed :(
	if err != 0 {
		return out[0:out_size], Errno(err)
	}

	return out[0:out_size], nil
}

// Decompress decompresses the byte array b passed in into the byte array o, and returns the size of the valid uncompressed data.
// If o is not large enough to hold the  compressed data, an error is returned.
func (z *Compressor) Decompress(b []byte, o []byte) (uint, error) {

	// both and input param (size of 'o') and output param (decompressed size)
	out_size := uint(len(o))

	err := C.lzo1x_decompress((*C.uchar)(unsafe.Pointer(&b[0])), C.lzo_uint(len(b)),
		(*C.uchar)(unsafe.Pointer(&o[0])), (*C.lzo_uint)(unsafe.Pointer(&out_size)), nil)

	// decompression failed :(
	if err != 0 {
		return out_size, Errno(err)
	}

	return out_size, nil
}

// for an input of n, what is the worst-case compression we might get
func lzo1x_1_output_size(n int) int {
	return (n + n/16 + 64 + 3)
}

// wrap the C calls so we can store a function pointer to them
func lzo1x_1_compress(b []byte, out []byte, out_size *int, wrkmem []byte) C.int {
	return C.lzo1x_1_compress((*C.uchar)(unsafe.Pointer(&b[0])), C.lzo_uint(len(b)),
		(*C.uchar)(unsafe.Pointer(&out[0])), (*C.lzo_uint)(unsafe.Pointer(out_size)),
		unsafe.Pointer(&wrkmem[0]))
}

func lzo1x_999_compress(b []byte, out []byte, out_size *int, wrkmem []byte) C.int {
	return C.lzo1x_999_compress((*C.uchar)(unsafe.Pointer(&b[0])), C.lzo_uint(len(b)),
		(*C.uchar)(unsafe.Pointer(&out[0])), (*C.lzo_uint)(unsafe.Pointer(out_size)),
		unsafe.Pointer(&wrkmem[0]))
}
