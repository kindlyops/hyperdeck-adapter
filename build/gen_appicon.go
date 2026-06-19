//go:build ignore

// Command gen_appicon renders the application icon used for packaging:
//   - icon/appicon.png   1024x1024 source (macOS .icns is derived from it in CI)
//   - icon/app.ico       multi-size Windows icon (installer + future exe resource)
//
// Run via `go generate ./build/...`. The artwork is a dark rounded tile with a
// centered green transport disc, matching the system-tray "locked" glyph.
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

var (
	tile  = color.NRGBA{0x23, 0x27, 0x2e, 0xff} // dark slate background
	disc  = color.NRGBA{0x2e, 0xcc, 0x71, 0xff} // green transport disc
	clear = color.NRGBA{0, 0, 0, 0}
)

// lerp blends a over b by alpha (0..1) for anti-aliased edges.
func lerp(a, b color.NRGBA, t float64) color.NRGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	bl := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t) }
	return color.NRGBA{bl(b.R, a.R), bl(b.G, a.G), bl(b.B, a.B), bl(b.A, a.A)}
}

// render draws the icon at size px with a supersampled, anti-aliased result.
func render(size int) image.Image {
	const ss = 4
	w := size * ss
	radius := float64(w) * 0.2237 // macOS-style corner radius
	discR := float64(w) * 0.30
	cx, cy := float64(w)/2, float64(w)/2
	img := image.NewNRGBA(image.Rect(0, 0, w, w))
	for y := 0; y < w; y++ {
		for x := 0; x < w; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			// Rounded-rect coverage (signed distance to the rounded tile).
			cover := roundedRectCoverage(px, py, float64(w), radius)
			c := lerp(tile, clear, cover)
			// Disc on top.
			d := math.Hypot(px-cx, py-cy)
			discCover := clampCover(discR - d)
			c = lerp(disc, c, discCover)
			img.SetNRGBA(x, y, c)
		}
	}
	return downsample(img, size, ss)
}

// roundedRectCoverage returns ~1 inside the rounded square of side w (origin 0),
// ~0 outside, with a 1px feather at the edge.
func roundedRectCoverage(px, py, w, r float64) float64 {
	// Distance from point to the rounded-rect border (negative inside).
	hx, hy := w/2, w/2
	qx := math.Abs(px-hx) - (hx - r)
	qy := math.Abs(py-hy) - (hy - r)
	ax, ay := math.Max(qx, 0), math.Max(qy, 0)
	dist := math.Hypot(ax, ay) + math.Min(math.Max(qx, qy), 0) - r
	return clampCover(-dist)
}

// clampCover maps a signed distance (px) to 0..1 coverage with a ~1px feather.
func clampCover(d float64) float64 {
	return math.Max(0, math.Min(1, d+0.5))
}

func downsample(src *image.NRGBA, size, ss int) image.Image {
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					p := src.NRGBAAt(x*ss+sx, y*ss+sy)
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

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

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
	binary.Write(&dir, binary.LittleEndian, uint16(0))
	binary.Write(&dir, binary.LittleEndian, uint16(1))
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
		dir.WriteByte(0)
		dir.WriteByte(0)
		binary.Write(&dir, binary.LittleEndian, uint16(1))
		binary.Write(&dir, binary.LittleEndian, uint16(32))
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
	if err := os.MkdirAll("icon", 0o755); err != nil {
		panic(err)
	}
	if err := writePNG("icon/appicon.png", render(1024)); err != nil {
		panic(err)
	}
	var ico []image.Image
	for _, s := range []int{16, 24, 32, 48, 64, 128, 256} {
		ico = append(ico, render(s))
	}
	if err := writeICO("icon/app.ico", ico); err != nil {
		panic(err)
	}
}
