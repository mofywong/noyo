package core

import (
	"bytes"
	"errors"
	"golang.org/x/image/draw"
	"image"
	"image/png"
	"io"
	"net/http"
)

// Only runs after the same revision and scope checks as the full-size image.
// Derive the preview from validated storage, never a browser-supplied URL/path.
func screenWriteThumbnail(w http.ResponseWriter, source io.ReadSeeker) error {
	config, _, err := image.DecodeConfig(source)
	if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 40_000_000 {
		return errors.New("Invalid image / 图片无效")
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return err
	}
	original, _, err := image.Decode(source)
	if err != nil {
		return err
	}
	width, height := config.Width, config.Height
	if width > 256 || height > 256 {
		if width >= height {
			height = max(1, height*256/width)
			width = 256
		} else {
			width = max(1, width*256/height)
			height = 256
		}
	}
	preview := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(preview, preview.Bounds(), original, original.Bounds(), draw.Src, nil)
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, preview); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "image/png")
	_, err = w.Write(encoded.Bytes())
	return err
}
