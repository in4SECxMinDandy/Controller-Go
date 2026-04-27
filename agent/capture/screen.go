// Package capture: chụp màn hình primary và encode JPEG.
package capture

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/kbinani/screenshot"
	"golang.org/x/image/draw"
)

// CaptureOpts cấu hình cho 1 lần chụp.
type CaptureOpts struct {
	Quality int     // JPEG quality 1..100
	Scale   float64 // 1.0 = giữ nguyên; 0.5 = giảm còn nửa mỗi chiều
}

// CaptureJPEG chụp màn hình display 0 và trả về JPEG bytes.
// Nếu opts.Scale < 1.0 sẽ downscale trước khi encode để giảm CPU.
func CaptureJPEG(opts CaptureOpts) ([]byte, error) {
	n := screenshot.NumActiveDisplays()
	if n < 1 {
		return nil, fmt.Errorf("no active display")
	}
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("capture: %w", err)
	}

	var src image.Image = img
	if opts.Scale > 0 && opts.Scale < 1.0 {
		w := int(float64(img.Bounds().Dx()) * opts.Scale)
		h := int(float64(img.Bounds().Dy()) * opts.Scale)
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		// ApproxBiLinear nhanh hơn BiLinear/CatmullRom, đủ đẹp cho streaming.
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		src = dst
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: opts.Quality}); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}
