//go:build ignore

// Command gen_icons renders the system-tray icons (idle/locked) as multi-size
// PNG-in-ICO files. Run via `go generate ./internal/adapter/driven/tray/...`.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// render draws a transparent-background disc glyph at size px. When filled, the
// disc is solid; otherwise it is a ring. col is the glyph color.
func render(size int, col color.NRGBA, filled bool) image.Image {
	const ss = 4 // supersample for anti-aliasing
	w := size * ss
	img := image.NewNRGBA(image.Rect(0, 0, w, w))
	cx, cy := float64(w)/2, float64(w)/2
	outer := float64(w) * 0.42
	inner := outer * 0.55 // ring thickness for the idle glyph
	for y := 0; y < w; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Hypot(dx, dy)
			var on bool
			if filled {
				on = d <= outer
			} else {
				on = d <= outer && d >= inner
			}
			if on {
				img.SetNRGBA(x, y, col)
			}
		}
	}
	// Downsample by averaging ss x ss blocks for smooth edges.
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					p := img.NRGBAAt(x*ss+sx, y*ss+sy)
					r += int(p.R)
					g += int(p.G)
					b += int(p.B)
					a += int(p.A)
				}
			}
			n := ss * ss
			out.SetNRGBA(x, y, color.NRGBA{uint8(r / n), uint8(g / n), uint8(b / n), uint8(a / n)})
		}
	}
	return out
}

// writeICO packs PNG-encoded images into a single .ico file.
func writeICO(path string, imgs []image.Image) error {
	var dir bytes.Buffer
	var blobs [][]byte
	for _, im := range imgs {
		var b bytes.Buffer
		if err := png.Encode(&b, im); err != nil {
			return err
		}
		blobs = append(blobs, b.Bytes())
	}
	binary.Write(&dir, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&dir, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&dir, binary.LittleEndian, uint16(len(imgs)))
	offset := 6 + 16*len(imgs)
	for i, im := range imgs {
		b := im.Bounds()
		wb, hb := byte(b.Dx()), byte(b.Dy())
		if b.Dx() >= 256 {
			wb = 0
		}
		if b.Dy() >= 256 {
			hb = 0
		}
		dir.WriteByte(wb)
		dir.WriteByte(hb)
		dir.WriteByte(0)                                    // palette
		dir.WriteByte(0)                                    // reserved
		binary.Write(&dir, binary.LittleEndian, uint16(1))  // planes
		binary.Write(&dir, binary.LittleEndian, uint16(32)) // bpp
		binary.Write(&dir, binary.LittleEndian, uint32(len(blobs[i])))
		binary.Write(&dir, binary.LittleEndian, uint32(offset))
		offset += len(blobs[i])
	}
	out := append([]byte{}, dir.Bytes()...)
	for _, blob := range blobs {
		out = append(out, blob...)
	}
	return os.WriteFile(path, out, 0o644)
}

func main() {
	sizes := []int{16, 24, 32, 48}
	// Locked: solid green disc. Idle: gray ring. Both transparent background so
	// they read on light and dark Windows taskbars.
	green := color.NRGBA{0x2e, 0xcc, 0x71, 0xff}
	gray := color.NRGBA{0x9e, 0x9e, 0x9e, 0xff}

	var locked, idle []image.Image
	for _, s := range sizes {
		locked = append(locked, render(s, green, true))
		idle = append(idle, render(s, gray, false))
	}
	if err := writeICO("icon_locked.ico", locked); err != nil {
		panic(err)
	}
	if err := writeICO("icon_idle.ico", idle); err != nil {
		panic(err)
	}
}
