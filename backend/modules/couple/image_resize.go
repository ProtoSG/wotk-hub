package couple

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// Every uploaded photo is normalized to two JPEG variants on upload — see
// resizeToJPEG below. This keeps storage/serving down to one content-type to
// reason about (JPEG) regardless of what format was uploaded, and guarantees
// consistent presigned-URL behavior for both the full and thumbnail object.
const (
	fullMaxDim  = 2000 // longest side, in px, for the full-size variant
	thumbMaxDim = 400  // longest side, in px, for the thumbnail variant
	jpegQuality = 85
)

// decodeImage decodes r according to contentType, which must already be one
// of the keys in allowedPhotoTypes — the caller validates that before
// calling in.
//
// GIF note: image/gif.Decode only reads the first frame as a static image
// (no animated-GIF support here) — an uploaded animated GIF becomes a single
// static JPEG after resizing. Acceptable tradeoff: this feature has no need
// for animation.
func decodeImage(r io.Reader, contentType string) (image.Image, error) {
	switch contentType {
	case "image/jpeg":
		return jpeg.Decode(r)
	case "image/png":
		return png.Decode(r)
	case "image/webp":
		return webp.Decode(r)
	case "image/gif":
		return gif.Decode(r)
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// resizeToJPEG scales img down so its longest side is at most maxDim,
// preserving aspect ratio, then re-encodes the result as a JPEG at
// jpegQuality. Never upscales: if img is already within maxDim on its
// longest side, it's encoded as-is.
func resizeToJPEG(img image.Image, maxDim int) ([]byte, error) {
	src := img.Bounds()
	w, h := src.Dx(), src.Dy()

	longest := w
	if h > longest {
		longest = h
	}

	out := img
	if longest > maxDim {
		scale := float64(maxDim) / float64(longest)
		newW := max(1, int(float64(w)*scale+0.5))
		newH := max(1, int(float64(h)*scale+0.5))
		dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, src, draw.Over, nil)
		out = dst
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
