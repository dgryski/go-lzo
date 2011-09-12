// Example program for lzo.go
// This program is based heavily off the 'lzopack' example from the LZO library
// License: GPLv3 or later
// Copyright (C) 2011 Damian Gryski <damian@gryski.com>

package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"hash/adler32"
	"io"
	"_obj/lzo"
	"os"
)

var MagicHeader [7]byte = [...]byte{0x00, 0xe9, 0x4c, 0x5a, 0x4f, 0xff, 0x1a}

func read8(r io.Reader) uint {
	var u uint8
	err := binary.Read(r, binary.BigEndian, &u)
	if err != nil {
		fatal(fmt.Sprint("read failed: ", err))
	}
	return uint(u)
}

func read32(r io.Reader) uint {
	var u uint32
	err := binary.Read(r, binary.BigEndian, &u)
	if err != nil {
		fatal(fmt.Sprint("read failed: ", err))
	}
	return uint(u)
}

func write8(w io.Writer, c byte) {
	err := binary.Write(w, binary.BigEndian, c)
	if err != nil {
		fatal(fmt.Sprint("write failed: ", err))
	}
}

func write32(w io.Writer, ui uint32) {
	err := binary.Write(w, binary.BigEndian, ui)
	if err != nil {
		fatal(fmt.Sprint("write failed: ", err))
	}
}

func fatal(s string) {
	fmt.Fprintln(os.Stderr, "FATAL: ", s)
	os.Exit(1)
}

func do_compress(in *os.File, out *os.File, level uint, blocksize uint) {

	out.Write(MagicHeader[:])

	write32(out, 1)                // flags
	write8(out, 1)                 // method
	write8(out, uint8(level&0xff)) // level
	write32(out, uint32(blocksize))

	h := adler32.New()

	var algorithm lzo.LzoAlgorithm

	if level == 1 {
		algorithm = lzo.BestSpeed
	} else {
		algorithm = lzo.BestCompression
	}

	z, _ := lzo.NewCompressor(algorithm)

	inb := make([]byte, blocksize)

	for {
		nr, err := in.Read(inb[:])

		if nr == 0 && err == os.EOF {
			break
		}

		if err != nil {
			fatal(fmt.Sprint("read failed: ", err))
		}

		// update checksum
		h.Write(inb[:nr])

		// try to compress
		o, _ := z.Compress(inb[:nr])

		compressed_size := len(o)

		// we didn't compress it
		if compressed_size > nr {
			write32(out, uint32(nr))
			write32(out, uint32(nr))
			out.Write(inb[:nr])
		} else {
			write32(out, uint32(nr))
			write32(out, uint32(compressed_size))
			out.Write(o)
		}
	}

	// eof marker
	write32(out, 0)

	// is this right?

	hashb := h.Sum()
	sum := (uint32(hashb[0]) << 24) | (uint32(hashb[1]) << 16) | (uint32(hashb[2]) << 8) | (uint32(hashb[3]) << 0)

	binary.Write(out, binary.BigEndian, sum)

	return
}

func do_decompress(in *os.File, out *os.File) {

	var magic [7]byte

	in.Read(magic[:])

	if bytes.Compare(magic[:], MagicHeader[:]) != 0 {
		fatal("header error -- this file was not compressed with lzopack")
	}

	flags := read32(in)
	method := read8(in)
	level := read8(in)
	block_size := read32(in)

	if method != 1 {
		fatal(fmt.Sprint("header error - unknown compression method: ", method, " (level: ", level, ")"))
	}

	if block_size < 1024 || block_size > 8*1024*1024 {
		fatal(fmt.Sprint("header error -- invalid block size: ", block_size))
	}

	z, _ := lzo.NewCompressor(lzo.Lzo1x_1)
	h := adler32.New()
	inb := make([]byte, block_size+block_size/16+64+3)

	for {

		uncompressed_blocksize := read32(in)

		// end of compressed blocks?
		if uncompressed_blocksize == 0 {
			break
		}

		outb := make([]byte, uncompressed_blocksize)

		compressed_blocksize := read32(in)
		nr, err := in.Read(inb[:compressed_blocksize])

		if nr == 0 {
			fatal("unexpected end of file")
		}

		if err != nil {
			fatal(fmt.Sprint(err))
		}

		if compressed_blocksize == uncompressed_blocksize {
			// data was uncompressible -- nothing to decompress
			out.Write(inb[:compressed_blocksize])
			h.Write(inb[:compressed_blocksize])
			continue
		}

		sz, err := z.Decompress(inb[:compressed_blocksize], outb)

		if sz != uncompressed_blocksize || err != nil {
			fatal("compressed data violation")
		}

		out.Write(outb[:sz])
		h.Write(outb[:sz]) // update hash
	}

	if flags&1 == 1 {
		checksum := read32(in)

		hashb := h.Sum()
		sum := (uint(hashb[0]) << 24) | (uint(hashb[1]) << 16) | (uint(hashb[2]) << 8) | (uint(hashb[3]) << 0)

		if checksum != sum {
			fatal("checksum error -- data corrupted")
		}
	}
}

func main() {

	//        var flag_block_size *int = flag.Int("block-size", 256*1024, "block size to use for compression")
        blocksize := uint(256 * 1024);

	var flag_best_speed *bool = flag.Bool("1", false, "Use fastest compression algorithm")
	var flag_best_compression *bool = flag.Bool("9", false, "Use best compression algorithm")
	var flag_decompress *bool = flag.Bool("d", false, "Decompress")

	flag.Parse()

	if flag.NArg() != 2 {
		fmt.Println("usage: lzopack [ -1 | -9 ] [ -d ] in-file out-file")
		os.Exit(1)
	}

	var level uint

	if *flag_best_speed && *flag_best_compression {
		fmt.Fprintln(os.Stderr, "Can only specify one of -1 -9")
		os.Exit(1)
	} else if *flag_best_speed {
		level = 1
	} else if *flag_best_compression {
		level = 9
	} else {
		// default
		level = 9
	}

	if *flag_best_speed && *flag_best_compression && *flag_decompress {
		// passing compression levels when decompressing is pointless
	}

	in_file, err := os.Open(flag.Arg(0))
	if err != nil {
		fatal(fmt.Sprint("error opening input file: ", flag.Arg(0), ": ", err))
	}

	out_file, err := os.Create(flag.Arg(1))

	if err != nil {
		fatal(fmt.Sprint("error opening output file: ", flag.Arg(1), ": ", err))
	}

	if *flag_decompress {
		do_decompress(in_file, out_file)
	} else {
		do_compress(in_file, out_file, level, blocksize);
	}

	out_file.Close()

}
