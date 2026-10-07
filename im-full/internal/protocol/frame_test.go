package protocol

import (
	"bytes"
	"testing"
	"testing/iotest"
)

func TestRoundTrip(t *testing.T) {
	in := &Frame{Cmd: CmdSendReq, Seq: 42, Body: []byte("hello")}
	out, err := ReadFrame(bytes.NewReader(in.Marshal()), DefaultMaxBody)
	if err != nil {
		t.Fatal(err)
	}
	if out.Cmd != in.Cmd || out.Seq != in.Seq || string(out.Body) != "hello" {
		t.Fatalf("mismatch: %+v", out)
	}
}

// 半包：每次 Read 只返回 1 字节，ReadFull 必须能拼完整。
func TestHalfPacket(t *testing.T) {
	in := &Frame{Cmd: CmdPush, Seq: 7, Body: bytes.Repeat([]byte("x"), 300)}
	out, err := ReadFrame(iotest.OneByteReader(bytes.NewReader(in.Marshal())), DefaultMaxBody)
	if err != nil || len(out.Body) != 300 {
		t.Fatalf("err=%v out=%+v", err, out)
	}
}

// 粘包：多个帧连续到达，应按边界逐个切出。
func TestStickyPackets(t *testing.T) {
	var buf []byte
	for i := uint32(1); i <= 3; i++ {
		buf = (&Frame{Cmd: CmdHeartbeat, Seq: i}).AppendTo(buf)
	}
	r := bytes.NewReader(buf)
	for i := uint32(1); i <= 3; i++ {
		f, err := ReadFrame(r, DefaultMaxBody)
		if err != nil || f.Seq != i {
			t.Fatalf("i=%d f=%+v err=%v", i, f, err)
		}
	}
}

func TestRejectOversizeAndShort(t *testing.T) {
	big := (&Frame{Cmd: 1, Body: make([]byte, 100)}).Marshal()
	if _, err := ReadFrame(bytes.NewReader(big), 50); err != ErrFrameTooLarge {
		t.Fatalf("want ErrFrameTooLarge, got %v", err)
	}
	short := []byte{0, 0, 0, 2, 0, 1, 0, 0, 0, 0} // length=2 < 6
	if _, err := ReadFrame(bytes.NewReader(short), 50); err != ErrFrameTooShort {
		t.Fatalf("want ErrFrameTooShort, got %v", err)
	}
	// length=0xFFFFFFFF 不能触发巨量分配
	evil := []byte{0xff, 0xff, 0xff, 0xff, 0, 1, 0, 0, 0, 0}
	if _, err := ReadFrame(bytes.NewReader(evil), DefaultMaxBody); err != ErrFrameTooLarge {
		t.Fatalf("want ErrFrameTooLarge, got %v", err)
	}
}
