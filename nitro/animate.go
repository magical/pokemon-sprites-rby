package nitro

import (
	"fmt"
	"os"

	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/magical/gif"
)

const doScale = true

type Animation struct {
	ncgr *NCGR
	nclr *NCLR
	ncer *NCER
	nanr *NANR
	nmcr *NMCR
	nmar *NMAR

	pal        color.Palette
	cells      []image.Image // cache of rendered cells
	cellBounds []image.Rectangle //cache of cell bounds
}

func NewAnimation(
	ncgr *NCGR,
	nclr *NCLR,
	ncer *NCER,
	nanr *NANR,
	nmcr *NMCR,
	nmar *NMAR,
) *Animation {
	a := new(Animation)
	a.ncgr = ncgr
	a.nclr = nclr
	a.ncer = ncer
	a.nanr = nanr
	a.nmcr = nmcr
	a.nmar = nmar

	a.pal = nclr.Palette(0)
	a.pal[0] = setTrans(a.pal[0])
	for i, c := range a.pal {
		a.pal[i] = color.NRGBAModel.Convert(c)
	}
	fmt.Fprintln(os.Stderr, a.pal)

	// Cache the cells
	cells := make([]image.Image, ncer.Len())
	cellBounds := make([]image.Rectangle, ncer.Len())
	for i := range cells {
		m := image.Image(ncer.Cell(i, ncgr, a.pal))
		r := m.Bounds()
		if doScale {
			m = scale8x(m)
		}
		r.Max.X = max(abs(r.Min.X), abs(r.Max.X))
		r.Max.Y = max(abs(r.Min.Y), abs(r.Max.Y))
		r.Min.X = -r.Max.X
		r.Min.Y = -r.Max.Y
		cells[i] = m
		cellBounds[i] = r.Canon()
	}
	a.cells = cells
	a.cellBounds = cellBounds

	//a.pal[0] = setOpaque(a.pal[0])

	return a
}

func abs(x int) int {
	if x < 0 {
		x = -x
	}
	return x
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func setTrans(c color.Color) color.Color {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	n.A = 0
	return n
}

func setOpaque(c color.Color) color.Color {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	n.A = 255
	return n
}

type point struct {
	X float64
	Y float64
}

type transform struct {
	Rotate float64
	ScaleX float64
	ScaleY float64
}

// MAcell -> MFrame -> Mcell -> Mobj -> Acell -> Frame -> Cell -> OBJ

func (a *Animation) renderMAcell(dst draw.Image, dp image.Point, c Acell, t int) {
	f := c.FrameAt(t)
	//dp = dp.Add(rotatePoint(f.X, f.Y, f.Rotate))
	dp.X += f.X
	dp.Y += f.Y
	tr := transform{
		Rotate: float64(f.Rotate) / 65536,
		ScaleX: float64(f.ScaleX) / 4096,
		ScaleY: float64(f.ScaleY) / 4096,
	}
	if f.Cell < a.nmcr.Len() {
		a.renderMcell(dst, dp, a.nmcr.Mcell(f.Cell), tr, t)
	}
}

func rotatePoint(x, y int, r float64) image.Point {
	if r == 0 {
		return image.Point{x, y}
	}
	return point{float64(x), float64(y)}.Rotate(r).Int()
}

func (p point) Add(q point) point {
	p.X += q.X
	p.Y += q.Y
	return p
}

func (p point) Rotate(deg float64) point {
	sin := math.Sin(deg * (2*math.Pi))
	cos := math.Cos(deg * (2*math.Pi))
	p.X = p.X*cos + p.Y*sin
	p.Y = p.Y*cos - p.X*sin
	return p
}

func (p point) Int() image.Point {
	return image.Pt(int(p.X), int(p.Y))
}

func (a *Animation) renderMcell(dst draw.Image, dp image.Point, objs []mobj, tr transform, t int) {
	for _, obj := range objs {
		if int(obj.AcellIndex) < len(a.nanr.Cells) {
			a.renderAcell(
				dst,
				dp.Add(rotatePoint(int(obj.X), int(obj.Y), -tr.Rotate)),
				a.nanr.Cells[obj.AcellIndex],
				tr,
				t,
			)
		}
	}
}

func (a *Animation) renderAcell(dst draw.Image, dp image.Point, c Acell, tr transform, t int) {
	f := c.FrameAt(t)
	dp = dp.Add(rotatePoint(f.X, f.Y, -tr.Rotate))
	//dp = dp.Add(image.Pt(f.X, f.Y))
	tr.Rotate += float64(f.Rotate) / 65536
	tr.ScaleX *= float64(f.ScaleX) / 4096
	tr.ScaleY *= float64(f.ScaleY) / 4096
	if f.Cell <= len(a.cells) {
		a.drawCell(dst, dp, f.Cell, tr)
	}
}

func (a *Animation) drawCell(dst draw.Image, dp image.Point, i int, tr transform) {
	if doScale {
		rotate(dst, a.cellBounds[i].Add(dp), dp, a.cells[i], image.ZP, 8/tr.ScaleX, 8/tr.ScaleY, tr.Rotate*360)
	} else {
		rotate(dst, a.cellBounds[i].Add(dp), dp, a.cells[i], image.ZP, 1/tr.ScaleX, 1/tr.ScaleY, tr.Rotate*360)
		//drawUnder(dst, a.cellBounds[i].Add(dp), a.cells[i], a.cellBounds[i].Min)
	}
}

// Returns the Frame at time t.
func (a *Acell) FrameAt(t int) Frame {
	// TODO: Handle PlayMode
	total := 0
	for _, f := range a.Frames {
		if t < int(f.Duration) {
			return f
		}
		t -= int(f.Duration)
		total += int(f.Duration)
	}
	t = t % total
	for i := 0; i < 100; i++ {
		for _, f := range a.Frames[a.LoopStart:] {
			if t < int(f.Duration) {
				return f
			}
			t -= int(f.Duration)
		}
	}
	panic("infinite loop")
}

// Render renders a single frame.
func (a *Animation) RenderFrame(t int) *image.Paletted {
	r := image.Rect(0, 0, 192, 96)
	rgba := image.NewRGBA(r)
	a.renderMAcell(rgba, image.Pt(196/2, 96), a.nmar.Cells[0], t)

	p := image.NewPaletted(rgba.Bounds(), a.pal)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p.SetColorIndex(x, y, paletteIndex(a.pal, rgba.At(x, y)))
		}
	}
	return p
}

func paletteIndex(p color.Palette, c color.Color) uint8 {
	cr, cg, cb, ca := c.RGBA()
	if ca == 0 {
		return 0
	}
	for i, v := range p {
		vr, vg, vb, va := v.RGBA()
		if cr == vr && cg == vg && cb == vb && ca == va {
			return uint8(i)
		}
	}
	return uint8(p.Index(c))
}

// Render draws every frame and returns a GIF.
func (a *Animation) Render() *gif.GIF {
	g := new(gif.GIF)
	t := 0
	total := 0
	for _, f := range a.nmar.Cells[0].Frames {
		total += int(f.Duration)
	}
	//total = 100
	for t < total {
		fmt.Fprintln(os.Stderr, "time", t)
		p := a.RenderFrame(t)
		tt := a.nextFrame(t)
		g.Image = append(g.Image, p)
		//g.Delay = []int{0}
		g.Delay = append(g.Delay, tt*100/60-t*100/60)
		g.Disposal = append(g.Disposal, gif.DisposeBackground)
		t = tt
	}
	return g
}

func (a *Animation) nextFrame(t int) int {
	/*
	least := -1
	mac := a.nmar.Cells[0]
	mf := mac.FrameAt(t)
	mf.Duration
	mobjs := a.nmcr.Mcell(mf.Cell)
	for _, mobj := range mobjs {
		ac := a.nanr.Cells[mobj.AcellIndex]
		f := ac.FrameAt(t)

	for i, m := range  {
		for _, 

	}
	*/
	//for i, c := range a.nmar.Cells { }
	return t + 1
}

/*
func (a *Acell) remain(t int) int {
	total := 0
	for i, f := range a.Frames {
		if t < int(f.Duration) {
			return int(f.Duration) - t
		}
		t -= int(f.Duration)
		if i >= a.LoopStart {
			total += int(f.Duration)
		}
	}
	t = t % total
	for i := 0; i < 100; i++ {
		for _, f := range a.Frames[a.LoopStart:] {
			if t < int(f.Duration) {
				return int(f.Duration) - t
			}
			t -= int(f.Duration)
		}
	}
	panic("infinite loop")
}
*/
