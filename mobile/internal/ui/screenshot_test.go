package ui

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestPrepareScreenshot(t *testing.T) {
	encode := func(w, h int) []byte {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	small := encode(400, 800)
	data, _, err := prepareScreenshot(small)
	if err != nil || !bytes.Equal(data, small) {
		t.Fatalf("a small screenshot should be sent as it is (err %v)", err)
	}
	data, img, err := prepareScreenshot(encode(1200, 3200))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 600 || b.Dy() != 1600 {
		t.Fatalf("scaled to %v, want 600x1600", b.Size())
	}
	if !bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) {
		t.Fatal("a scaled screenshot should be sent as JPEG")
	}
	if _, _, err := prepareScreenshot([]byte("not an image")); err == nil {
		t.Fatal("a file that is not an image should be refused")
	}
}
