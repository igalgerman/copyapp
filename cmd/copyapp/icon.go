package main

import (
	"bytes"
	"encoding/binary"
	"math"
)

// makeICO draws a 32x32 rounded blue square with two overlapping white
// "pages" and returns it as a BMP-based .ico (what the Windows tray loads).
func makeICO() []byte {
	const n = 32
	px := make([][4]byte, n*n) // BGRA, top-down
	set := func(x, y int, b, g, r, a byte) {
		if x >= 0 && x < n && y >= 0 && y < n {
			px[y*n+x] = [4]byte{b, g, r, a}
		}
	}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			cx := math.Min(math.Max(float64(x), 7), n-1-7)
			cy := math.Min(math.Max(float64(y), 7), n-1-7)
			if math.Hypot(float64(x)-cx, float64(y)-cy) <= 7 {
				set(x, y, 0xD7, 0x78, 0x1E, 0xFF)
			}
		}
	}
	rect := func(x0, y0, x1, y1 int, b, g, r byte) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				set(x, y, b, g, r, 0xFF)
			}
		}
	}
	rect(7, 6, 19, 22, 0xB0, 0xB0, 0xB0)
	rect(8, 7, 18, 21, 0xE8, 0xE8, 0xE8)
	rect(12, 10, 24, 26, 0xFF, 0xFF, 0xFF)
	for _, y := range []int{14, 17, 20} {
		rect(14, y, 22, y+1, 0xD7, 0x78, 0x1E)
	}

	var dib bytes.Buffer
	w := func(v any) { binary.Write(&dib, binary.LittleEndian, v) }
	w(uint32(40))
	w(int32(n))
	w(int32(n * 2)) // XOR + AND masks
	w(uint16(1))
	w(uint16(32))
	w(uint32(0))
	w(uint32(n * n * 4))
	w(int32(0))
	w(int32(0))
	w(uint32(0))
	w(uint32(0))
	for y := n - 1; y >= 0; y-- { // bottom-up
		for x := 0; x < n; x++ {
			dib.Write(px[y*n+x][:])
		}
	}
	dib.Write(make([]byte, (n/8)*n)) // AND mask; alpha channel rules

	var out bytes.Buffer
	o := func(v any) { binary.Write(&out, binary.LittleEndian, v) }
	o(uint16(0))
	o(uint16(1))
	o(uint16(1))
	out.Write([]byte{n, n, 0, 0})
	o(uint16(1))
	o(uint16(32))
	o(uint32(dib.Len()))
	o(uint32(22))
	out.Write(dib.Bytes())
	return out.Bytes()
}
