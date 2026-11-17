package ui

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	"io"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/x/explorer"
	xdraw "golang.org/x/image/draw"
)

// upiNote is the note on the UPI payment: the Student ID once the student has one, else their email.
func (a *App) upiNote() string {
	if code := a.me.Student.Code(); code != "" {
		return code
	}
	return a.me.Student.Email
}

// Screenshots are sent as compact JPEGs (they upload, and later load for the admin, quickly).
// Only a JPEG that is already this small is sent as it is.
const (
	maxShotBytes = 400 << 10
	maxShotSide  = 1400
)

// pickScreenshot lets the student choose the payment screenshot from the phone.
func (a *App) pickScreenshot() {
	go func() {
		r, err := a.expl.ChooseFile(".png", ".jpg", ".jpeg", ".webp")
		if err != nil {
			if !errors.Is(err, explorer.ErrUserDecline) {
				a.post(func() { a.toast("Could not open the file: "+err.Error(), true) })
			}
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r, 30<<20))
		r.Close()
		var data []byte
		var img image.Image
		if err == nil {
			data, img, err = prepareScreenshot(raw)
		}
		a.post(func() {
			if err != nil {
				a.toast("Could not use this image. Choose a PNG or JPG screenshot.", true)
				return
			}
			if s := a.checkout; s != nil {
				s.shot, s.shotPreview, s.err = data, paint.NewImageOp(img), ""
			}
		})
	}()
}

// prepareScreenshot checks the file is an image and returns the bytes to upload (the original
// when it is small enough, else a smaller JPEG) and the decoded image for the preview.
func prepareScreenshot(raw []byte) ([]byte, image.Image, error) {
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, err
	}
	b := img.Bounds()
	if len(raw) <= maxShotBytes && b.Dx() <= maxShotSide && b.Dy() <= maxShotSide && format == "jpeg" {
		return raw, img, nil
	}
	scale := min(1, float64(maxShotSide)/float64(max(b.Dx(), b.Dy())))
	dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(b.Dx())*scale)), max(1, int(float64(b.Dy())*scale))))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src) // transparent areas become white
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), dst, nil
}

// screenshotPicker is the "Payment screenshot" box on the details step.
func (a *App) screenshotPicker(gtx C) D {
	s := a.checkout
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return a.smallLabel(gtx, "Payment screenshot") }),
		layout.Rigid(vspace(6)),
		layout.Rigid(func(gtx C) D {
			return s.pickShot.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return withBackground(gtx, func(gtx C, size image.Point) {
					fillRound(gtx, size, 14, a.th.P.Lav)
					strokeRound(gtx, size, 14, 1.5, a.th.P.Violet)
				}, func(gtx C) D {
					return pad(14).Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								if len(s.shot) == 0 {
									return iconSized(gtx, icUpload, 28, a.th.P.Violet)
								}
								sz := s.shotPreview.Size()
								h := gtx.Dp(170)
								w := max(1, sz.X*h/max(1, sz.Y))
								return imageBox(gtx, s.shotPreview, image.Pt(min(w, gtx.Constraints.Max.X), h), 10)
							}),
							layout.Rigid(vspace(8)),
							layout.Rigid(func(gtx C) D {
								t := "Upload payment screenshot"
								if len(s.shot) > 0 {
									t = "Screenshot added · tap to change"
								}
								return centered(gtx, t, a.th, textOpt{size: 14, weight: font.Bold, color: a.th.P.Accent})
							}),
						)
					})
				})
			})
		}),
		layout.Rigid(vspace(6)),
		layout.Rigid(func(gtx C) D {
			return a.th.muted(gtx, "The \"payment successful\" screen from your UPI app. It is deleted once your payment is verified.", 12)
		}),
	)
}
