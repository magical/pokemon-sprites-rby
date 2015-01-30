package nitro

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/magical/gif"
)

type Animation struct {
	ncgr *NCGR
	nclr *NCLR
	ncer *NCER
	nanr *NANR
	//nmar *NMAR
	//nmcr *NMCR

	pal color.Palette

	// Cache of rendered cells
	cells []image.Image
}

func NewAnimation(
	ncgr *NCGR,
	nclr *NCLR,
	ncer *NCER,
	nanr *NANR,
) *Animation {
	a := new(Animation)
	a.ncgr = ncgr
	a.nclr = nclr
	a.ncer = ncer
	a.nanr = nanr

	a.pal = nclr.Palette(0)
	a.pal[0] = setTrans(a.pal[0])

	// Cache the cells
	cells := make([]image.Image, ncer.Len())
	for i := range cells {
		m := ncer.Cell(i, ncgr, a.pal)
		big := scale8x(m)
		cells[i] = big
	}
	a.cells = cells

	return a
}

func setTrans(c color.Color) color.Color {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	n.A = 0
	return n
}

// DrawFrame renders a single frame of a cell.
func (a *Animation) drawFrame(dst draw.Image, dp image.Point, f Frame) {
	c := a.cells[f.Cell]
	//rotate(dst, dst.Bounds(), dp, c, image.ZP, 8/float64(f.ScaleX), 8/float64(f.ScaleY), float64(f.Rotate)/65536*360)
	rotate(dst, dst.Bounds(), dp, c, image.ZP, 8*4096/float64(f.ScaleX), 8*4096/float64(f.ScaleY), -float64(f.Rotate)/65536*360)
	//draw.Draw(dst, c.Bounds().Add(dp), c, c.Bounds().Min, draw.Over)
}

// Render draws every frame and returns a GIF.
func (a *Animation) Render() *gif.GIF {
	g := new(gif.GIF)
	t := 0
	for i := range a.nanr.Acells {
		for _, f := range a.nanr.Acells[i].Frames {
			m := image.NewPaletted(image.Rect(0, 0, 64, 64), a.pal)
			a.drawFrame(m, image.Pt(f.X+32, f.Y+32), f)
			g.Image = append(g.Image, m)
			g.Delay = append(g.Delay, (t+int(f.Duration))*100/60 - t*100/60)
			t += int(f.Duration)
		}
	}
	return g
}
