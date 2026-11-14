package app

import (
	"io"
	"sync/atomic"
	"time"
)

// 备份与恢复时统计读写字节数、定时上报进度的小工具。

// countingWriter 统计写出的字节数。
type countingWriter struct {
	w io.Writer
	n atomic.Int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n.Add(int64(n))
	return n, err
}

type countingReader struct {
	r io.Reader
	n atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n.Add(int64(n))
	return n, err
}

// watch 每隔一会儿报告一次计数，直到 stop 被关闭。
func watch(stop <-chan struct{}, count func() int64, report func(int64)) {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			report(count())
		}
	}
}
