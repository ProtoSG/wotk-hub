package couple

import (
	"image"
	"image/color"
	"testing"
)

// markerImage returns a 3x2 (w=3, h=2) RGBA image with a single distinct red
// marker pixel at the given position and black everywhere else — small
// enough to eyeball, asymmetric enough (3≠2, one unique marker) to catch a
// mirrored or off-by-one transform that a symmetric test image would hide.
func markerImage(markerX, markerY int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			img.Set(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	img.Set(markerX, markerY, color.RGBA{255, 0, 0, 255})
	return img
}

// findMarker returns the coordinates of the (255,0,0) pixel in img.
func findMarker(t *testing.T, img image.Image) (int, int) {
	t.Helper()
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r>>8 == 255 && g>>8 == 0 && bl>>8 == 0 {
				return x - b.Min.X, y - b.Min.Y
			}
		}
	}
	t.Fatal("marker pixel not found")
	return -1, -1
}

// Marker at top-left (0,0) of a 3-wide, 2-tall source image. Expected
// destinations below are worked out by physically rotating/flipping a 3x2
// rectangle with a mark in its top-left corner, per the standard EXIF
// Orientation semantics (see applyOrientation's doc comment).
func TestApplyOrientation(t *testing.T) {
	cases := []struct {
		name          string
		orientation   int
		wantW, wantH  int
		wantX, wantY  int
	}{
		{"1 normal", 1, 3, 2, 0, 0},
		{"2 mirror horizontal", 2, 3, 2, 2, 0},
		{"3 rotate 180", 3, 3, 2, 2, 1},
		{"4 mirror vertical", 4, 3, 2, 0, 1},
		{"6 rotate 90 CW", 6, 2, 3, 1, 0},
		{"8 rotate 90 CCW", 8, 2, 3, 0, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := markerImage(0, 0)
			out := applyOrientation(src, c.orientation)
			b := out.Bounds()
			if b.Dx() != c.wantW || b.Dy() != c.wantH {
				t.Fatalf("bounds = %dx%d, want %dx%d", b.Dx(), b.Dy(), c.wantW, c.wantH)
			}
			x, y := findMarker(t, out)
			if x != c.wantX || y != c.wantY {
				t.Errorf("marker at (%d,%d), want (%d,%d)", x, y, c.wantX, c.wantY)
			}
		})
	}
}

func TestApplyOrientationNoOpForUnknown(t *testing.T) {
	src := markerImage(1, 1)
	out := applyOrientation(src, 1)
	if out != image.Image(src) {
		t.Error("orientation 1 should return the source image unchanged, not a copy")
	}
	out = applyOrientation(src, 0) // out-of-range, same as "no tag"
	if out != image.Image(src) {
		t.Error("out-of-range orientation should be a no-op")
	}
}
