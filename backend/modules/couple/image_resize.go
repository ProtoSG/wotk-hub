package couple

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/rwcarlsen/goexif/exif"
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

// exifOrientation reads the EXIF Orientation tag from data (raw upload
// bytes, not the decoded image), returning 1 (no correction needed) if the
// file carries no EXIF at all — true for PNG/WebP/GIF, and for JPEGs from
// sources that strip metadata. This is deliberately best-effort: any parse
// error is treated the same as "no orientation tag", never a hard failure —
// a photo failing to upload because its EXIF was malformed would be a much
// worse outcome than it just uploading unrotated.
func exifOrientation(data []byte) int {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	o, err := tag.Int(0)
	if err != nil || o < 1 || o > 8 {
		return 1
	}
	return o
}

// applyOrientation returns img with pixels physically rotated/flipped
// according to the EXIF Orientation convention (1-8; see
// https://exiftool.org/TagNames/EXIF.html#Orientation), so the result looks
// correct with no orientation metadata attached — which matters here because
// resizeToJPEG's output never carries EXIF (image/jpeg.Encode doesn't write
// any). Without this step, phone photos taken in any rotation other than the
// sensor's native one silently come out sideways/upside-down/mirrored once
// re-encoded, because the correction the phone recorded gets dropped instead
// of applied. orientation 1 (already correct) and any out-of-range value are
// a no-op, returned as-is.
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return flipH(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipV(img)
	case 5:
		return rotate90CW(flipH(img))
	case 6:
		return rotate90CW(img)
	case 7:
		return rotate90CCW(flipH(img))
	case 8:
		return rotate90CCW(img)
	default:
		return img
	}
}

func flipH(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.X-1-(x-b.Min.X)+b.Min.X, y, img.At(x, y))
		}
	}
	return dst
}

func flipV(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, b.Max.Y-1-(y-b.Min.Y)+b.Min.Y, img.At(x, y))
		}
	}
	return dst
}

func rotate180(img image.Image) image.Image {
	return flipV(flipH(img))
}

// rotate90CW rotates 90° clockwise, swapping width and height.
func rotate90CW(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// rotate90CCW rotates 90° counter-clockwise, swapping width and height.
func rotate90CCW(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
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
