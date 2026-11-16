package ui

import (
	"bytes"
	"image"
	"image/jpeg"
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
	var small bytes.Buffer
	if err := jpeg.Encode(&small, image.NewRGBA(image.Rect(0, 0, 400, 800)), nil); err != nil {
		t.Fatal(err)
	}
	data, _, err := prepareScreenshot(small.Bytes())
	if err != nil || !bytes.Equal(data, small.Bytes()) {
		t.Fatalf("a small JPEG should be sent as it is (err %v)", err)
	}
	data, _, err = prepareScreenshot(encode(400, 800))
	if err != nil || !bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) {
		t.Fatalf("a PNG screenshot should be sent as JPEG (err %v)", err)
	}
	data, img, err := prepareScreenshot(encode(1050, 2800))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 525 || b.Dy() != 1400 {
		t.Fatalf("scaled to %v, want 525x1400", b.Size())
	}
	if !bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) {
		t.Fatal("a scaled screenshot should be sent as JPEG")
	}
	if _, _, err := prepareScreenshot([]byte("not an image")); err == nil {
		t.Fatal("a file that is not an image should be refused")
	}
}
