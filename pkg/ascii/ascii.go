package ascii

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"unicode/utf8"
)

func NewWriter(w io.Writer, foreground, background, text string) io.Writer {
	// once clear screen
	_, _ = w.Write([]byte(csiClear))

	// every frame - move to home
	a := &writer{wr: w, buf: []byte(csiHome)}

	var idx0 uint8

	// https://en.wikipedia.org/wiki/ANSI_escape_code
	switch foreground {
	case "8":
		a.color = func(r, g, b uint8) {
			if idx := xterm256color(r, g, b, 8); idx != idx0 {
				idx0 = idx
				a.buf = append(a.buf, fmt.Sprintf("\033[%dm", 30+idx)...)
			}
		}
	case "256":
		a.color = func(r, g, b uint8) {
			if idx := xterm256color(r, g, b, 255); idx != idx0 {
				idx0 = idx
				a.buf = append(a.buf, fmt.Sprintf("\033[38;5;%dm", idx)...)
			}
		}
	case "rgb":
		a.color = func(r, g, b uint8) {
			a.buf = append(a.buf, fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)...)
		}
	}

	switch background {
	case "8":
		a.color = func(r, g, b uint8) {
			if idx := xterm256color(r, g, b, 8); idx != idx0 {
				idx0 = idx
				a.buf = append(a.buf, fmt.Sprintf("\033[%dm", 40+idx)...)
			}
		}
	case "256":
		a.color = func(r, g, b uint8) {
			if idx := xterm256color(r, g, b, 255); idx != idx0 {
				idx0 = idx
				a.buf = append(a.buf, fmt.Sprintf("\033[48;5;%dm", idx)...)
			}
		}
	case "rgb":
		a.color = func(r, g, b uint8) {
			a.buf = append(a.buf, fmt.Sprintf("\033[48;2;%d;%d;%dm", r, g, b)...)
		}
	}

	if len(text) == 1 {
		// fast 1 symbol version
		a.text = func(_, _, _ uint32) {
			a.buf = append(a.buf, text[0])
		}
	} else {
		switch text {
		case "":
			text = ` .::--~~==++**##%%$@` // default for empty text
		case "block":
			text = " ░░▒▒▓▓█" // https://en.wikipedia.org/wiki/Block_Elements
		}

		if runes := []rune(text); len(runes) != len(text) {
			k := float32(len(runes)-1) / 255
			a.text = func(r, g, b uint32) {
				i := gray(r, g, b, k)
				a.buf = utf8.AppendRune(a.buf, runes[i])
			}
		} else {
			k := float32(len(text)-1) / 255
			a.text = func(r, g, b uint32) {
				i := gray(r, g, b, k)
				a.buf = append(a.buf, text[i])
			}
		}
	}

	return a
}

type writer struct {
	wr    io.Writer
	buf   []byte
	color func(r, g, b uint8)
	text  func(r, g, b uint32)
}

// https://stackoverflow.com/questions/37774983/clearing-the-screen-by-printing-a-character
const csiClear = "\033[2J"
const csiHome = "\033[H"

func (a *writer) Write(p []byte) (n int, err error) {
	img, err := jpeg.Decode(bytes.NewReader(p))
	if err != nil {
		return 0, err
	}

	a.buf = a.buf[:len(csiHome)]

	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if a.color != nil {
				a.color(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			}
			a.text(r, g, b)
		}
		a.buf = append(a.buf, '\n')
	}

	a.buf = append(a.buf, "\033[0m"...)

	if n, err = a.wr.Write(a.buf); err == nil {
		a.wr.(http.Flusher).Flush()
	}

	return
}

func gray(r, g, b uint32, k float32) uint8 {
	gr := (19595*r + 38470*g + 7471*b + 1<<15) >> 24 // uint8
	return uint8(float32(gr) * k)
}

const x256r = "\x00\x80\x00\x80\x00\x80\x00\xc0\x80\xff\x00\xff\x00\xff\x00\xff\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x5f\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\x87\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xaf\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xd7\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\x08\x12\x1c\x26\x30\x3a\x44\x4e\x58\x60\x66\x76\x80\x8a\x94\x9e\xa8\xb2\xbc\xc6\xd0\xda\xe4\xee"
const x256g = "\x00\x00\x80\x80\x00\x00\x80\xc0\x80\x00\xff\xff\x00\x00\xff\xff\x00\x00\x00\x00\x00\x00\x5f\x5f\x5f\x5f\x5f\x5f\x87\x87\x87\x87\x87\x87\xaf\xaf\xaf\xaf\xaf\xaf\xd7\xd7\xd7\xd7\xd7\xd7\xff\xff\xff\xff\xff\xff\x00\x00\x00\x00\x00\x00\x5f\x5f\x5f\x5f\x5f\x5f\x87\x87\x87\x87\x87\x87\xaf\xaf\xaf\xaf\xaf\xaf\xd7\xd7\xd7\xd7\xd7\xd7\xff\xff\xff\xff\xff\xff\x00\x00\x00\x00\x00\x00\x5f\x5f\x5f\x5f\x5f\x5f\x87\x87\x87\x87\x87\x87\xaf\xaf\xaf\xaf\xaf\xaf\xd7\xd7\xd7\xd7\xd7\xd7\xff\xff\xff\xff\xff\xff\x00\x00\x00\x00\x00\x00\x5f\x5f\x5f\x5f\x5f\x5f\x87\x87\x87\x87\x87\x87\xaf\xaf\xaf\xaf\xaf\xaf\xd7\xd7\xd7\xd7\xd7\xd7\xff\xff\xff\xff\xff\xff\x00\x00\x00\x00\x00\x00\x5f\x5f\x5f\x5f\x5f\x5f\x87\x87\x87\x87\x87\x87\xaf\xaf\xaf\xaf\xaf\xaf\xd7\xd7\xd7\xd7\xd7\xd7\xff\xff\xff\xff\xff\xff\x00\x00\x00\x00\x00\x00\x5f\x5f\x5f\x5f\x5f\x5f\x87\x87\x87\x87\x87\x87\xaf\xaf\xaf\xaf\xaf\xaf\xd7\xd7\xd7\xd7\xd7\xd7\xff\xff\xff\xff\xff\xff\x08\x12\x1c\x26\x30\x3a\x44\x4e\x58\x60\x66\x76\x80\x8a\x94\x9e\xa8\xb2\xbc\xc6\xd0\xda\xe4\xee"
const x256b = "\x00\x00\x00\x00\x80\x80\x80\xc0\x80\x00\x00\x00\xff\xff\xff\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x00\x5f\x87\xaf\xd7\xff\x08\x12\x1c\x26\x30\x3a\x44\x4e\x58\x60\x66\x76\x80\x8a\x94\x9e\xa8\xb2\xbc\xc6\xd0\xda\xe4\xee"

func xterm256color(r, g, b uint8, n int) (index uint8) {
	best := uint16(0xFFFF)
	for i := 0; i < n; i++ {
		diff := uint16(r-x256r[i]) + uint16(g-x256g[i]) + uint16(b-x256b[i])
		if diff < best {
			best = diff
			index = uint8(i)
		}
	}
	return
}
