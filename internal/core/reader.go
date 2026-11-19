package core

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// lineReader 直接借用 bufio 的缓冲返回普通行，长行则拼接到可复用的临时缓冲。
// 返回的字节只在下一次读取前有效，需要保留时由调用方复制。
type lineReader struct {
	reader   *bufio.Reader
	longLine []byte
	prefix   []byte
}

func newLineReader(source io.Reader) *lineReader {
	return &lineReader{reader: bufio.NewReader(source), longLine: []byte{}, prefix: []byte{}}
}

func (r *lineReader) reset(source io.Reader) {
	// 切换输入源并重置读取位置，不重新分配普通读缓冲；长行缓冲也只清空长度。
	r.reader.Reset(source)
	r.prefix = r.prefix[:0]
	r.longLine = r.longLine[:0]
}

func (r *lineReader) release() {
	// 解除对旧输入源和探测字节的引用，避免 reader 跨文件持有它们。
	r.reader.Reset(nil)
	r.prefix = nil
	// 只保留容量不超过上限的长行缓冲，避免一次异常长行导致长期占用大块内存。
	if cap(r.longLine) > 64*1024 {
		r.longLine = nil
	}
}

func (r *lineReader) next() ([]byte, error) {
	// 先逐行回放语言探测已经读走的字节，消费完后再从 bufio 继续读取。
	if len(r.prefix) > 0 {
		length := len(r.prefix)
		if end := bytes.IndexByte(r.prefix, '\n'); end >= 0 {
			length = end + 1
		}
		line := r.prefix[:length]
		r.prefix = r.prefix[length:]
		return line, nil
	}
	line, err := r.reader.ReadSlice('\n')
	if !errors.Is(err, bufio.ErrBufferFull) {
		return line, err
	}
	// 一行装不进读缓冲时，先复制已读片段，再继续读取，避免片段被覆盖。
	r.longLine = append(r.longLine[:0], line...)
	for errors.Is(err, bufio.ErrBufferFull) {
		line, err = r.reader.ReadSlice('\n')
		r.longLine = append(r.longLine, line...)
	}
	return r.longLine, err
}

func containsCommentBytes(line []byte, multiLines [][]string) bool {
	for _, comments := range multiLines {
		for _, comment := range comments {
			if bytes.Contains(line, []byte(comment)) {
				return true
			}
		}
	}
	return false
}
