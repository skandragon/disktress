package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"golang.org/x/crypto/blake2b"
)

var (
	seed       = flag.String("seed", "abcdefghijk", "Seed for 'random' data written to disk")
	blocks     = flag.Int64("blocks", -1, "number of blocks to use")
	blocksize  = flag.Int64("blocksize", 512, "block size, in bytes.  Must be a multiple of 64.")
	filename   = flag.String("filename", "tmpfile", "target filename.  Intent is to write to a raw disk.")
	startblock = flag.Int64("startblock", 0, "offset block to start writing and verifying.")
	mode       = flag.String("mode", "rw", "rw = read+verify, r = verify, w = write")
	iterations = flag.Int64("iterations", 1, "number of times to repeat the test")

	sourcecount int64
)

func makeblock(seed string, blocksize int64, block int64) []byte {
	key := fmt.Sprintf("%s.%d", seed, block)
	sha, err := blake2b.New512([]byte(key))
	if err != nil {
		panic(err)
	}
	buffer := make([]byte, blocksize)
	for i := int64(0); i < blocksize/blake2b.Size; i++ {
		off := i * blake2b.Size
		b := sha.Sum([]byte{})
		copy(buffer[off:], b)
		sha.Write(b)
	}
	return buffer
}

func writeblock(f *os.File, sources []chan []byte, blocksize int64, block int64) {
	data := <-sources[block%sourcecount]
	written, err := f.WriteAt(data, block*blocksize)
	if err != nil {
		panic(err)
	}
	if written != len(data) {
		panic(fmt.Errorf("partial write (%d written, expected to write %d", written, len(data)))
	}
}

func checkblock(f *os.File, sources []chan []byte, blocksize int64, block int64) {
	data := <-sources[block%sourcecount]
	contents := make([]byte, len(data))
	read, err := f.ReadAt(contents, block*blocksize)
	if read != len(data) {
		panic(fmt.Errorf("partial read (%d read, expected to read %d", read, len(data)))
	}
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(data, contents) {
		panic(fmt.Errorf("block %d failed to compare", block))
	}
}

func generator(c chan []byte, block int64, count int64) {
	for i := block; i < count; i += sourcecount {
		c <- makeblock(*seed, *blocksize, block)
	}
	close(c)
}

func startGenerators(sources []chan []byte, startblock int64, count int64) {
	for i := int64(0); i < sourcecount; i++ {
		go generator(sources[(int64(i)+startblock)%sourcecount], startblock+int64(i), count)
	}
}

func main() {
	flag.Parse()

	if *blocks <= 0 {
		fmt.Fprintf(os.Stderr, "blocks must be > 0\n")
		os.Exit(1)
	}

	if *blocksize%64 != 0 {
		fmt.Fprintf(os.Stderr, "blocksize must be a multiple of 64\n")
		os.Exit(1)
	}

	sourcecount = int64(runtime.NumCPU())
	if sourcecount > *blocks {
		sourcecount = *blocks
	}

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
			sources := make([]chan []byte, sourcecount)
			for i := int64(0); i < sourcecount; i++ {
				sources[i] = make(chan []byte)
			}
			startGenerators(sources, *startblock, *blocks)
			for i := *startblock; i < *blocks; i++ {
				duration := measure(func() {
					writeblock(f, sources, *blocksize, i)
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
			sources := make([]chan []byte, sourcecount)
			for i := int64(0); i < sourcecount; i++ {
				sources[i] = make(chan []byte)
			}
			startGenerators(sources, *startblock, *blocks)
			for i := *startblock; i < *blocks; i++ {
				duration := measure(func() {
					checkblock(f, sources, *blocksize, i)
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
