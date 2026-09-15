package core

import (
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

const maxImageUploadBytes = 10 << 20

// saveUploadedImage validates actual content, limits decoded allocation and
// strips active content/metadata by encoding to a uniquely named PNG.
func saveUploadedImage(file *ghttp.UploadFile, directory string) (string, error) {
	if file == nil || file.Size <= 0 || file.Size > maxImageUploadBytes {
		return "", errors.New("image exceeds 10 MB limit")
	}
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	return storeDecodedImage(src, directory)
}

func storeDecodedImage(src io.ReadSeeker, directory string) (string, error) {
	cfg, _, err := image.DecodeConfig(io.LimitReader(src, maxImageUploadBytes+1))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 25_000_000 {
		return "", errors.New("invalid image or image dimensions too large")
	}
	if _, err = src.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	img, _, err := image.Decode(io.LimitReader(src, maxImageUploadBytes+1))
	if err != nil {
		return "", errors.New("invalid image content")
	}
	if err = os.MkdirAll(directory, 0750); err != nil {
		return "", err
	}
	dst, err := os.CreateTemp(directory, "image-*.png")
	if err != nil {
		return "", err
	}
	name := dst.Name()
	if err = png.Encode(dst, img); err != nil {
		dst.Close()
		os.Remove(name)
		return "", err
	}
	if err = dst.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return filepath.Base(name), nil
}

func openStoredFile(directory, name string) (*os.File, error) {
	// Flat object names only: reject both separators regardless of host OS.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00") {
		return nil, os.ErrNotExist
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(name) // Also prevents symlink escapes and check/open races.
}

func serveStoredFile(r *ghttp.Request, directory, prefix string, attachment bool) {
	name := strings.TrimPrefix(r.URL.Path, prefix)
	f, err := openStoredFile(directory, name)
	if err != nil {
		r.Response.WriteStatus(http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		r.Response.WriteStatus(http.StatusNotFound)
		return
	}
	header := r.Response.Header()
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cache-Control", "private, no-store")
	header.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	contentType := "application/octet-stream"
	if !attachment {
		_, format, decodeErr := image.DecodeConfig(io.LimitReader(f, maxImageUploadBytes))
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			r.Response.WriteStatus(http.StatusNotFound)
			return
		}
		if decodeErr == nil {
			switch format {
			case "jpeg", "png", "gif", "webp", "bmp":
				contentType = "image/" + format
			}
		}
	}
	if attachment || contentType == "application/octet-stream" {
		header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	}
	header.Set("Content-Type", contentType)
	http.ServeContent(r.Response.Writer, r.Request, name, info.ModTime(), f)
}
