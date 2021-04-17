package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"time"
)

var (
	seed       = flag.String("seed", "abcdefghijk", "Seed for 'random' data written to disk")
	blocks     = flag.Int64("blocks", -1, "number of blocks to use")
	blocksize  = flag.Int64("blocksize", 512, "block size, in bytes.  Must be a multiple of 32.")
	filename   = flag.String("filename", "tmpfile", "target filename.  Intent is to write to a raw disk.")
	startblock = flag.Int64("startblock", 0, "offset block to start writing and verifying.")
	mode       = flag.String("mode", "rw", "rw = read+verify, r = verify, w = write")
	iterations = flag.Int64("iterations", 1, "number of times to repeat the test")

	buffer []byte
)

func makeblock(seed string, blocksize int64, block int64) {
	sha := sha256.New()
	_, err := sha.Write([]byte(seed))
	if err != nil {
		panic(err)
	}
	_, err = sha.Write([]byte(fmt.Sprintf("%d", block)))
	if err != nil {
		panic(err)
	}
	for i := int64(0); i < blocksize/(256/8); i++ {
		off := i * 256 / 8
		b := sha.Sum([]byte{})
		copy(buffer[off:], b)
		sha.Write(b)
	}
}

func writeblock(f *os.File, seed string, blocksize int64, block int64) {
	makeblock(seed, blocksize, block)
	written, err := f.WriteAt(buffer, block*blocksize)
	if err != nil {
		panic(err)
	}
	if written != len(buffer) {
		panic(fmt.Errorf("partial write (%d written, expected to write %d", written, len(buffer)))
	}
}

func checkblock(f *os.File, seed string, blocksize int64, block int64) {
	makeblock(seed, blocksize, block)
	contents := make([]byte, len(buffer))
	read, err := f.ReadAt(contents, block*blocksize)
	if read != len(buffer) {
		panic(fmt.Errorf("partial read (%d read, expected to read %d", read, len(buffer)))
	}
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(buffer, contents) {
		panic(fmt.Errorf("block %d failed to compare", block))
	}
}

func main() {
	flag.Parse()

	if *blocks <= 0 {
		fmt.Fprintf(os.Stderr, "blocks must be > 0\n")
		os.Exit(1)
	}

	if *blocksize%(256/32) != 0 {
		fmt.Fprintf(os.Stderr, "blocksize must be a multiple of 32\n")
		os.Exit(1)
	}

	buffer = make([]byte, *blocksize)
	interval := int64(10000)
	if *blocksize > 100000 {
		interval = 100
	} else if *blocksize > 10000 {
		interval = 1000
	} else if *blocksize > 1000 {
		interval = 10000
	}

	for iteration := int64(0); iteration < *iterations; iteration++ {
		f, err := os.OpenFile(*filename, os.O_RDWR, 0600)
		if err != nil {
			panic(err)
		}

		if *mode == "rw" || *mode == "w" {
			var accum time.Duration
			blockCount := int64(0)
			for i := *startblock; i < *blocks; i++ {
				duration := measure(func() {
					writeblock(f, *seed, *blocksize, i)
				})
				accum += duration
				blockCount++
				accumBytes := blockCount * *blocksize
				accumSpeed := float32(accumBytes) / float32(accum.Seconds()) / 1048576.0
				instSpeed := float32(*blocksize) / float32(duration.Seconds()) / 1048576.0
				if i%interval == 0 && i != 0 {
					fmt.Printf("Wrote block %d, iteration %d/%d (%.2f%%, %.2f MB/sec, %.2f MB/sec overall)\n",
						i, iteration+1, *iterations, (float32(i) / float32(*blocks) * 100), instSpeed, accumSpeed)
				}
			}
		}

		if *mode == "rw" || *mode == "r" {
			var accum time.Duration
			blockCount := int64(0)
			for i := *startblock; i < *blocks; i++ {
				duration := measure(func() {
					checkblock(f, *seed, *blocksize, i)
				})
				accum += duration
				blockCount++
				accumBytes := blockCount * *blocksize
				accumSpeed := float32(accumBytes) / float32(accum.Seconds()) / 1048576.0
				instSpeed := float32(*blocksize) / float32(duration.Seconds()) / 1048576.0
				if i%interval == 0 && i != 0 {
					fmt.Printf("Verified block %d, iteration %d/%d (%.2f%%, %.2f MB/sec, %.2f MB/sec overall)\n",
						i, iteration+1, *iterations, (float32(i) / float32(*blocks) * 100), instSpeed, accumSpeed)
				}
			}
		}
		f.Close()
	}
}

type measureFunc func()

func measure(f measureFunc) time.Duration {
	startTime := time.Now()
	f()
	endTime := time.Now()
	return endTime.Sub(startTime)
}
