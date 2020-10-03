package main

import (
	"io"
)

type combiner struct {
	r io.Reader
	w io.Writer
}

func newReadWriter(r io.Reader, w io.Writer) io.ReadWriter {
	return &combiner{
		r: r,
		w: w,
	}
}

func (c *combiner) Read(p []byte) (n int, err error) {
	return c.r.Read(p)
}

func (c *combiner) Write(p []byte) (n int, err error) {
	return c.w.Write(p)
}
