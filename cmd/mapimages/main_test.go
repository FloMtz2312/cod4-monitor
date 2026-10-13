package main

import (
	"encoding/binary"
	"image/color"
	"testing"
)

func TestDecodeColorBlock(t *testing.T) {
	// c0 = pure red, c1 = pure blue (c0 > c1: 4-color mode); pixel n uses index n%4
	b := make([]byte, 8)
	binary.LittleEndian.PutUint16(b, 0xF800)
	binary.LittleEndian.PutUint16(b[2:], 0x001F)
	var idx uint32
	for n := range 16 {
		idx |= uint32(n%4) << (2 * n)
	}
	binary.LittleEndian.PutUint32(b[4:], idx)
	px := decodeColorBlock(b, true)
	want := []color.RGBA{{255, 0, 0, 255}, {0, 0, 255, 255}, {170, 0, 85, 255}, {85, 0, 170, 255}}
	for n := range 4 {
		if px[n] != want[n] || px[n+4] != want[n] {
			t.Fatalf("pixel %d = %v, want %v", n, px[n], want[n])
		}
	}
}

func TestDecodeIWI(t *testing.T) {
	// 4x4 DXT1 image: header (28 bytes) + one block, all red
	b := make([]byte, 28+8)
	copy(b, "IWi\x06\x0b")
	binary.LittleEndian.PutUint16(b[6:], 4)
	binary.LittleEndian.PutUint16(b[8:], 4)
	binary.LittleEndian.PutUint16(b[28:], 0xF800)
	img, err := decodeIWI(b)
	if err != nil {
		t.Fatal(err)
	}
	if r, g, bl, _ := img.At(3, 3).RGBA(); r>>8 != 255 || g != 0 || bl != 0 {
		t.Fatalf("got %v %v %v", r>>8, g>>8, bl>>8)
	}
	if _, err := decodeIWI([]byte("nope")); err == nil {
		t.Fatal("want error")
	}
}
