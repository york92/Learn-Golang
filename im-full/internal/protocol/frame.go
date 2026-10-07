// Package protocol 定义 IM 长连接的二进制帧格式。
//
//	+-----------+----------+----------+------------------+
//	| length 4B | cmd 2B   | seq 4B   | body (length-6)  |
//	+-----------+----------+----------+------------------+
//
// length = len(cmd)+len(seq)+len(body) = 6 + len(body)，大端序。
// TCP 是字节流，没有"消息边界"，所以必须靠 length 字段自己切包。
package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	// HeaderSize 是 length + cmd + seq 的总长度。
	HeaderSize = 10
	// innerHeader 是 length 字段所统计的头部部分（cmd + seq）。
	innerHeader = 6

	// DefaultMaxBody 单帧 body 上限。富媒体走对象存储，IM 通道只传小消息。
	DefaultMaxBody = 64 * 1024
)

var (
	ErrFrameTooShort = errors.New("protocol: frame length too short")
	ErrFrameTooLarge = errors.New("protocol: frame body too large")
)

// Frame 是一个完整的协议帧。
type Frame struct {
	Cmd  uint16
	Seq  uint32
	Body []byte
}

// AppendTo 把帧序列化后追加到 dst，便于复用缓冲区、避免多余分配。
func (f *Frame) AppendTo(dst []byte) []byte {
	dst = binary.BigEndian.AppendUint32(dst, uint32(innerHeader+len(f.Body)))
	dst = binary.BigEndian.AppendUint16(dst, f.Cmd)
	dst = binary.BigEndian.AppendUint32(dst, f.Seq)
	return append(dst, f.Body...)
}

// Marshal 返回独立的字节切片。
func (f *Frame) Marshal() []byte {
	return f.AppendTo(make([]byte, 0, HeaderSize+len(f.Body)))
}

// ReadFrame 从 r 读取一个完整帧。r 通常是 *bufio.Reader。
//
// 安全要点：必须先校验 length 再分配内存，否则攻击者发一个
// length=4GB 的头就能让服务端 OOM。
func ReadFrame(r io.Reader, maxBody int) (*Frame, error) {
	var hdr [HeaderSize]byte
	// io.ReadFull 会一直读到填满，天然处理"半包"。
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[0:4])
	if n < innerHeader {
		return nil, ErrFrameTooShort
	}
	if n-innerHeader > uint32(maxBody) {
		return nil, ErrFrameTooLarge
	}
	f := &Frame{
		Cmd: binary.BigEndian.Uint16(hdr[4:6]),
		Seq: binary.BigEndian.Uint32(hdr[6:10]),
	}
	if bodyLen := int(n - innerHeader); bodyLen > 0 {
		f.Body = make([]byte, bodyLen)
		if _, err := io.ReadFull(r, f.Body); err != nil {
			return nil, err
		}
	}
	return f, nil
}

// WriteFrame 一次性写出整帧（单次 Write，避免头和体被拆开交错）。
func WriteFrame(w io.Writer, f *Frame) error {
	_, err := w.Write(f.Marshal())
	return err
}
