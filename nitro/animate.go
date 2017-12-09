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
const usePaletted = true

const debug = false
const animIndex = 15

var red = color.NRGBA{0xff, 0, 0, 0xff}
var blue = color.NRGBA{0, 0, 0xff, 0xff}

type Animation struct {
	ncgr *NCGR
	nclr *NCLR
	ncer *NCER
	nanr *NANR
	nmcr *NMCR
	nmar *NMAR

	pal    color.Palette
	cell   []image.Image     // cache of rendered cells
	big    []image.Image     // cache of 8x cells
	bounds []image.Rectangle // cache of cell bounds
	state  []state           // state of animated cells
}

// State represents the current state of an animated cell
type state struct {
	frame  int // index of the current frame
	remain int // remaining time
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
	if !usePaletted {
		a.pal[0] = setTrans(a.pal[0])
	}
	if debug {
		a.pal = append(a.pal, color.Black, red, blue)
	}

	// Cache the cells
	a.cell = make([]image.Image, ncer.Len())
	a.bounds = make([]image.Rectangle, ncer.Len())
	if doScale {
		a.big = make([]image.Image, ncer.Len())
	}
	for i := range a.cell {
		m := image.Image(ncer.Cell(i, ncgr, a.pal))
		a.cell[i] = m
		a.bounds[i] = m.Bounds()
		if doScale {
			a.big[i] = scale8x(m)
		}
	}

	a.state = make([]state, len(nanr.Cells))

	return a
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

type transform struct {
	Rotate float64
	ScaleX float64
	ScaleY float64
}

var identity = transform{0, 1, 1}

// MAcell -> MFrame -> Mcell -> Mobj -> Acell -> Frame -> Cell -> OBJ

func (a *Animation) renderMAcell(dst draw.Image, dp image.Point, c Acell, t int) {
	f, _ := c.FrameAt(t)
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

func rotatePoint(x, y int, tr transform) image.Point {
	if tr == identity {
		return image.Pt(x, y)
	}
	sin := math.Sin(tr.Rotate * (2 * math.Pi))
	cos := math.Cos(tr.Rotate * (2 * math.Pi))
	x0 := float64(x)*tr.ScaleX
	y0 := float64(y)*tr.ScaleY
	x1 := x0*cos - y0*sin
	y1 := y0*cos + x0*sin
	return image.Pt(int(x1), int(y1))
}

func (a *Animation) renderMcell(dst draw.Image, dp image.Point, objs []mobj, tr transform, t int) {
	for _, obj := range objs {
		if debug && animIndex >= 0 && int(obj.AcellIndex) != animIndex {
			continue
		}
		if int(obj.AcellIndex) < len(a.nanr.Cells) {
			a.renderAcell(
				dst,
				dp,
				image.Pt(int(obj.X), int(obj.Y)),
				int(obj.AcellIndex),
				tr,
				t,
			)
		}
	}
}

func (a *Animation) renderAcell(dst draw.Image, dp, p image.Point, i int, tr transform, t int) {
	//f, _ := a.nanr.Cells[i].FrameAt(t)
	f := a.nanr.Cells[i].Frames[a.state[i].frame]
	//dp = dp.Add(rotatePoint(f.X+p.X, f.Y+p.Y, tr))
	dp = dp.Add(rotatePoint(f.X+p.X, f.Y+p.Y, tr))
	tr.Rotate += float64(f.Rotate) / 65536
	tr.ScaleX *= float64(f.ScaleX) / 4096
	tr.ScaleY *= float64(f.ScaleY) / 4096
	if f.Cell <= len(a.cell) {
		a.drawCell(dst, dp, f.Cell, tr)
	}
}

func (a *Animation) drawCell(dst draw.Image, dp image.Point, i int, tr transform) {
	r := a.bounds[i]
	if tr == identity {
		drawUnder(dst, r.Add(dp), a.cell[i], r.Min)
	} else if doScale {
		sp := image.ZP
		cp := r.Min.Add(r.Size().Div(2))
		if false {
			// Instead of rotating around (0,0), rotate around the center
			// of the image's bounds and adjust the coordinates to
			// compensate.
			sr := a.big[i].Bounds()
			sp = sr.Min.Add(sr.Size().Div(2))
			r = r.Sub(cp)
			dp = dp.Add(rotatePoint(cp.X, cp.Y, tr))
		} else {
			// Just move the viewport to where the image will be
			// after rotation.
			r = r.Sub(cp).Add(rotatePoint(cp.X, cp.Y, tr))
		}
		rotate(dst, r.Add(dp), dp, a.big[i], sp, 8/tr.ScaleX, 8/tr.ScaleY, tr.Rotate)
		//rotate(dst, double(r.Add(dp)), dp.Mul(2), a.big[i], sp, 4/tr.ScaleX, 4/tr.ScaleY, tr.Rotate)
		if debug {
			//drawPoint(dst, cp.Add(dp), red)                           // center of image
			//drawPoint(dst, rotatePoint(cp.X, cp.Y, tr).Add(dp), blue) // center after rotation
		}
	} else {
		cp := r.Min.Add(r.Size().Div(2))
		r = r.Sub(cp).Add(rotatePoint(cp.X, cp.Y, tr))
		rotate(dst, r.Add(dp), dp, a.cell[i], image.ZP, 1/tr.ScaleX, 1/tr.ScaleY, tr.Rotate)
	}
	if debug {
		drawBox(dst, r.Add(dp), color.Black)
		drawPoint(dst, dp, color.Black)
	}
}

func double(r image.Rectangle) image.Rectangle{
	r.Min = r.Min.Mul(2)
	r.Max = r.Max.Mul(2)
	return r
}

func drawBox(dst draw.Image, r image.Rectangle, c color.Color) {
	u := image.NewUniform(c)
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), u, image.ZP, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), u, image.ZP, draw.Src)
	draw.Draw(dst, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), u, image.ZP, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), u, image.ZP, draw.Src)
}

func drawPoint(dst draw.Image, p image.Point, c color.Color) {
	draw.Draw(dst, image.Rect(p.X-1, p.Y-1, p.X+2, p.Y+2), image.NewUniform(c), image.ZP, draw.Src)
}

func (s *state) reset(a *Acell) {
	s.frame = 0
	s.remain = a.Frames[0].Duration
}

// Advance moves the animation state forward by t ticks.
func (s *state) advance(a *Acell, t int) {
	for t >= s.remain {
		t -= s.remain
		s.frame += 1
		if s.frame == len(a.Frames) {
			s.frame = 0
		}
		s.remain = a.Frames[s.frame].Duration
	}
	s.remain -= t
}

// Returns the Frame at time t.
func (a *Acell) FrameAt(t int) (Frame, int) {
	// TODO: Handle PlayMode
	if a.PlayMode != 2 {
		panic(fmt.Sprintf("Playmode == %v", a.PlayMode))
	}
	total := 0
	for i, f := range a.Frames {
		if t < f.Duration {
			return f, t
		}
		t -= f.Duration
		if i >= a.LoopStart {
			total += f.Duration
		}
	}
	t = t % total
	for i := 0; i < 100; i++ {
		for _, f := range a.Frames[a.LoopStart:] {
			if t < f.Duration {
				return f, t
			}
			t -= f.Duration
		}
	}
	panic("infinite loop")
}

// Render renders a single frame.
func (a *Animation) RenderFrame(t int) *image.Paletted {
	r := image.Rect(0, 0, 144, 96)
	p := image.NewPaletted(r, a.pal)
	if usePaletted {
		a.renderMAcell(p, image.Pt(144/2, 96), a.nmar.Cells[0], t)
	} else {
		rgba := image.NewRGBA(r)
		a.renderMAcell(rgba, image.Pt(144/2, 96), a.nmar.Cells[0], t)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				p.SetColorIndex(x, y, paletteIndex(a.pal, rgba.At(x, y)))
			}
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
		total += f.Duration
	}
	for i := range a.state {
		a.state[i].reset(&a.nanr.Cells[i])
	}
	//total = 100
	for t < total {
		if debug {
			fmt.Fprintln(os.Stderr, "time", t)
		}
		p := a.RenderFrame(t)
		tt := a.nextFrame(t)
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, tt*100/60-t*100/60)
		g.Disposal = append(g.Disposal, gif.DisposeBackground)
		for i := range a.state {
			a.state[i].advance(&a.nanr.Cells[i], tt-t)
		}
		t = tt
	}
	return g
}

func (a *Animation) nextFrame(t int) int {
	f, tt := a.nmar.Cells[0].FrameAt(t)
	least := f.Duration - tt
	for _, s := range a.state {
		if least == -1 || (s.remain > 0 && s.remain < least) {
			least = s.remain
		}
	}
	if least <= 0 {
		least = 1
		//panic("no next frame")
	}
	return t + least
}
