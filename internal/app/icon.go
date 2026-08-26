package app

import "image"

// AppIcons 程序化绘制应用图标：深色圆角方块上一枚带环行星，
// 与浏览器深色 UI 同一视觉语言；无需随仓库维护图片资源。
func AppIcons() (big, small *image.NRGBA) {
	return drawAppIcon(32), drawAppIcon(16)
}

// 超采样倍率：内部按 SS 倍分辨率光栅化后盒式降采样，获得平滑边缘。
const iconSS = 4

func drawAppIcon(size int) *image.NRGBA {
	src := renderIconPixels(size * iconSS)
	out := image.NewNRGBA(image.Rect(0, 0, size, size))

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a uint32
			for dy := 0; dy < iconSS; dy++ {
				for dx := 0; dx < iconSS; dx++ {
					i := src.PixOffset(x*iconSS+dx, y*iconSS+dy)
					r += uint32(src.Pix[i+0])
					g += uint32(src.Pix[i+1])
					b += uint32(src.Pix[i+2])
					a += uint32(src.Pix[i+3])
				}
			}
			n := uint32(iconSS * iconSS)
			i := out.PixOffset(x, y)
			out.Pix[i+0] = byte(r / n)
			out.Pix[i+1] = byte(g / n)
			out.Pix[i+2] = byte(b / n)
			out.Pix[i+3] = byte(a / n)
		}
	}
	return out
}

// 图标用色
var (
	iconPlate   = [4]byte{30, 37, 52, 255}    // 圆角底板 深蓝灰
	iconPlanetA = [4]byte{116, 173, 255, 255} // 行星上半 亮蓝
	iconPlanetB = [4]byte{63, 118, 224, 255}  // 行星下半 深蓝
	iconRing    = [4]byte{232, 239, 250, 255} // 星环 近白
)

func renderIconPixels(s int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, s, s))

	cornerRadius := float64(s) * 0.24
	cx, cy := 0.5*float64(s), 0.53*float64(s)
	planetR := 0.28 * float64(s)
	ringRX := 0.42 * float64(s)
	ringRY := 0.15 * float64(s)
	ringInner := 0.86
	ringOuter := 1.14

	inRect := func(x, y float64) bool {
		if x < 0 || y < 0 || x >= float64(s) || y >= float64(s) {
			return false
		}
		cxr := cornerRadius - min2f(x, float64(s)-x)
		cyr := cornerRadius - min2f(y, float64(s)-y)
		if cxr <= 0 || cyr <= 0 {
			return true
		}
		return cxr*cxr+cyr*cyr <= cornerRadius*cornerRadius
	}

	for py := 0; py < s; py++ {
		fy := (float64(py) + 0.5) / float64(s)
		for px := 0; px < s; px++ {
			fx := (float64(px) + 0.5) / float64(s)
			x, y := fx*float64(s), fy*float64(s)

			var c [4]byte
			switch {
			case !inRect(x, y):
				// 全透明
			case inEllipse(x, y, cx, cy, ringRX*ringOuter, ringRY*ringOuter) &&
				!inEllipse(x, y, cx, cy, ringRX*ringInner, ringRY*ringInner):
				// 星环横穿行星之上，视觉上连贯
				c = iconRing
			case dist2(x, y, cx, cy) <= planetR*planetR:
				if y < cy {
					c = iconPlanetA
				} else {
					c = iconPlanetB
				}
			default:
				c = iconPlate
			}

			i := img.PixOffset(px, py)
			copy(img.Pix[i:i+4], c[:])
		}
	}
	return img
}

func inEllipse(x, y, cx, cy, rx, ry float64) bool {
	dx, dy := x-cx, y-cy
	return dx*dx/(rx*rx)+dy*dy/(ry*ry) <= 1
}

func dist2(x, y, cx, cy float64) float64 {
	dx, dy := x-cx, y-cy
	return dx*dx + dy*dy
}

func min2f(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
