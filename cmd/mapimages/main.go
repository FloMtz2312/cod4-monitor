// mapimages extracts the map load screens from a CoD4 install into JPEG thumbnails for the dashboard.
//
//	go run ./cmd/mapimages <cod4 dir> ui/public/maps
//
// Stock maps come from main/iw_*.iwd, custom maps from usermaps/<map>/*.iwd. Images are IWI v6 (DXT1/3/5).
package main

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const outW, outH = 480, 300 // load screens are square textures shown stretched to 16:10

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: mapimages <cod4 dir> <output dir>")
		os.Exit(2)
	}
	game, out := os.Args[1], os.Args[2]
	if err := os.MkdirAll(out, 0o755); err != nil {
		panic(err)
	}
	found := map[string][]byte{} // map name -> iwi data

	// stock: images/loadscreen_mp_<map>.iwi, later archives override earlier ones
	stock, _ := filepath.Glob(filepath.Join(game, "main", "iw_*.iwd"))
	sort.Strings(stock)
	stockRe := regexp.MustCompile(`^images/loadscreen_(mp_[a-z0-9_]+?)\.iwi$`)
	for _, f := range stock {
		eachFile(f, func(name string, read func() []byte) {
			if m := stockRe.FindStringSubmatch(strings.ToLower(name)); m != nil && !strings.HasSuffix(m[1], "_small") {
				found[m[1]] = read()
			}
		})
	}
	// custom: the biggest *loadscreen* image of the map's archives (names vary between mappers)
	dirs, _ := filepath.Glob(filepath.Join(game, "usermaps", "mp_*"))
	for _, d := range dirs {
		mapName := filepath.Base(d)
		iwds, _ := filepath.Glob(filepath.Join(d, "*.iwd"))
		for _, f := range iwds {
			eachFile(f, func(name string, read func() []byte) {
				n := strings.ToLower(name)
				if strings.HasPrefix(n, "images/") && strings.Contains(n, "loadscreen") && strings.HasSuffix(n, ".iwi") && !strings.Contains(n, "_small") {
					if b := read(); len(b) > len(found[mapName]) {
						found[mapName] = b
					}
				}
			})
		}
	}

	names := make([]string, 0, len(found))
	for n := range found {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		img, err := decodeIWI(found[n])
		if err != nil {
			fmt.Printf("skip %s: %v\n", n, err)
			continue
		}
		f, err := os.Create(filepath.Join(out, n+".jpg"))
		if err != nil {
			panic(err)
		}
		if err := jpeg.Encode(f, resize(img, outW, outH), &jpeg.Options{Quality: 78}); err != nil {
			panic(err)
		}
		f.Close()
		fmt.Println("ok", n)
	}
}

func eachFile(zipPath string, fn func(name string, read func() []byte)) {
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		fmt.Printf("skip %s: %v\n", zipPath, err)
		return
	}
	defer z.Close()
	for _, f := range z.File {
		fn(f.Name, func() []byte {
			r, err := f.Open()
			if err != nil {
				return nil
			}
			defer r.Close()
			b, _ := io.ReadAll(r)
			return b
		})
	}
}

// decodeIWI reads an IWI v6 texture (28-byte header, mip levels smallest first: the full image is at the end).
func decodeIWI(b []byte) (image.Image, error) {
	if len(b) < 28 || string(b[:3]) != "IWi" || b[3] != 6 {
		return nil, errors.New("not an IWI v6 file")
	}
	format := b[4]
	w, h := int(binary.LittleEndian.Uint16(b[6:])), int(binary.LittleEndian.Uint16(b[8:]))
	if format == 0x01 || format == 0x02 { // uncompressed ARGB32 / RGB24, stored B,G,R(,A)
		bpp := 4 - int(format-1)
		if len(b)-28 < w*h*bpp {
			return nil, errors.New("truncated IWI")
		}
		data := b[len(b)-w*h*bpp:]
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for p := range w * h {
			img.SetRGBA(p%w, p/w, color.RGBA{data[p*bpp+2], data[p*bpp+1], data[p*bpp], 255})
		}
		return img, nil
	}
	var blockSize int
	switch format {
	case 0x0B:
		blockSize = 8 // DXT1
	case 0x0C, 0x0D:
		blockSize = 16 // DXT3, DXT5
	default:
		return nil, fmt.Errorf("unsupported IWI format 0x%02x", format)
	}
	size := max(1, (w+3)/4) * max(1, (h+3)/4) * blockSize
	if len(b)-28 < size {
		return nil, errors.New("truncated IWI")
	}
	data := b[len(b)-size:]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	i := 0
	for by := 0; by < h; by += 4 {
		for bx := 0; bx < w; bx += 4 {
			block := data[i : i+blockSize]
			i += blockSize
			colorBlock := block[blockSize-8:] // DXT3/5: 8 alpha bytes first (ignored, load screens are opaque)
			px := decodeColorBlock(colorBlock, format == 0x0B)
			for y := 0; y < 4 && by+y < h; y++ {
				for x := 0; x < 4 && bx+x < w; x++ {
					img.SetRGBA(bx+x, by+y, px[y*4+x])
				}
			}
		}
	}
	return img, nil
}

// decodeColorBlock decodes the 8-byte color part of a DXT block into 16 pixels.
func decodeColorBlock(b []byte, dxt1 bool) [16]color.RGBA {
	c0, c1 := binary.LittleEndian.Uint16(b), binary.LittleEndian.Uint16(b[2:])
	rgb := func(c uint16) [3]int { // RGB565 -> 8 bits
		r, g, bl := int(c>>11&31), int(c>>5&63), int(c&31)
		return [3]int{r<<3 | r>>2, g<<2 | g>>4, bl<<3 | bl>>2}
	}
	p0, p1 := rgb(c0), rgb(c1)
	var pal [4][3]int
	pal[0], pal[1] = p0, p1
	for k := range 3 {
		if c0 > c1 || !dxt1 {
			pal[2][k] = (2*p0[k] + p1[k]) / 3
			pal[3][k] = (p0[k] + 2*p1[k]) / 3
		} else { // DXT1 3-color mode: index 3 is transparent, shown black
			pal[2][k] = (p0[k] + p1[k]) / 2
		}
	}
	idx := binary.LittleEndian.Uint32(b[4:])
	var px [16]color.RGBA
	for n := range 16 {
		c := pal[idx>>(2*n)&3]
		px[n] = color.RGBA{uint8(c[0]), uint8(c[1]), uint8(c[2]), 255}
	}
	return px
}

// resize scales with a box filter (averages every source pixel that falls in a destination pixel).
func resize(src image.Image, w, h int) image.Image {
	sb := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		y0, y1 := sb.Min.Y+y*sb.Dy()/h, sb.Min.Y+max((y+1)*sb.Dy()/h, y*sb.Dy()/h+1)
		for x := range w {
			x0, x1 := sb.Min.X+x*sb.Dx()/w, sb.Min.X+max((x+1)*sb.Dx()/w, x*sb.Dx()/w+1)
			var r, g, b, n uint32
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, _ := src.At(sx, sy).RGBA()
					r, g, b, n = r+cr>>8, g+cg>>8, b+cb>>8, n+1
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255})
		}
	}
	return dst
}
