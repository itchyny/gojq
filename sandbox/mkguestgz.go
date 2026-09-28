//go:build ignore

// Command mkguestgz gzip-compresses the guest wasm module for embedding. It is
// run by make build-sandbox. Using compress/gzip rather than the gzip command
// keeps the output byte-identical across platforms, so CI can verify that the
// committed module matches the source.
package main

import (
	"compress/gzip"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: mkguestgz input.wasm output.wasm.gz")
	}
	in, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	out, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	w, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := w.Write(in); err != nil {
		log.Fatal(err)
	}
	if err := w.Close(); err != nil {
		log.Fatal(err)
	}
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
}
