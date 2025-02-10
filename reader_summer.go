package gohtvfs

import "io"

type ReaderSummer struct {
	Reader io.Reader
	Sum    *int64
}

func (d *ReaderSummer) Read(p []byte) (int, error) {
	n, err := d.Reader.Read(p)
	*d.Sum += int64(n)
	return n, err
}

func NewSumReader(reader io.Reader, sum *int64) io.Reader {
	return &ReaderSummer{Reader: reader, Sum: sum}
}
