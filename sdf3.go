package sdf

import (
	"errors"
	"math"

	"github.com/soypat/sdf/internal/d2"
	"github.com/soypat/sdf/internal/d3"
	"gonum.org/v1/gonum/spatial/r2"
	"gonum.org/v1/gonum/spatial/r3"
)

// SDF3 is the interface to a 3d signed distance function object.
type SDF3 interface {
	Evaluate(p r3.Vec) float64
	BoundingBox() d3.Box
}

/*
func sdfBox3d(p, s r3.Vec) float64 {
	d := p.Abs().Sub(s)
	return d.Max(r3.Vec{0, 0, 0}).Length() + Min(d.MaxComponent(), 0)
}
*/

func sdfBox3d(p, s r3.Vec) float64 {
	d := r3.Sub(d3.AbsElem(p), s)
	if d.X > 0 && d.Y > 0 && d.Z > 0 {
		return r3.Norm(d)
	}
	if d.X > 0 && d.Y > 0 {
		return math.Hypot(d.X, d.Y) // V2{d.X, d.Y}.Length()
	}
	if d.X > 0 && d.Z > 0 {
		return math.Hypot(d.X, d.Z) // V2{d.X, d.Z}.Length()
	}
	if d.Y > 0 && d.Z > 0 {
		return math.Hypot(d.Y, d.Z) //V2{d.Y, d.Z}.Length()
	}
	if d.X > 0 {
		return d.X
	}
	if d.Y > 0 {
		return d.Y
	}
	if d.Z > 0 {
		return d.Z
	}
	return d3.Max(d)
}

// SorSDF3 solid of revolution, SDF2 to SDF3.
type SorSDF3 struct {
	sdf   SDF2
	theta float64 // angle for partial revolutions
	norm  r2.Vec  // pre-calculated normal to theta line
	bb    d3.Box
}

// RevolveTheta3D returns an SDF3 for a solid of revolution.
func RevolveTheta3D(sdf SDF2, theta float64) (SDF3, error) {
	if sdf == nil {
		return nil, nil
	}
	if theta < 0 {
		return nil, ErrMsg("theta < 0")
	}
	s := SorSDF3{}
	s.sdf = sdf
	// normalize theta
	s.theta = math.Mod(math.Abs(theta), tau)
	sin := math.Sin(s.theta)
	cos := math.Cos(s.theta)
	// pre-calculate the normal to the theta line
	s.norm = r2.Vec{-sin, cos}
	// work out the bounding box
	var vset d2.Set
	if s.theta == 0 {
		vset = []r2.Vec{{1, 1}, {-1, -1}}
	} else {
		vset = []r2.Vec{{0, 0}, {1, 0}, {cos, sin}}
		if s.theta > 0.5*pi {
			vset = append(vset, r2.Vec{0, 1})
		}
		if s.theta > pi {
			vset = append(vset, r2.Vec{-1, 0})
		}
		if s.theta > 1.5*pi {
			vset = append(vset, r2.Vec{0, -1})
		}
	}
	bb := s.sdf.BoundingBox()
	l := math.Max(math.Abs(bb.Min.X), math.Abs(bb.Max.X))
	vmin := r2.Scale(l, vset.Min())
	vmax := r2.Scale(l, vset.Max())
	s.bb = d3.Box{r3.Vec{vmin.X, vmin.Y, bb.Min.Y}, r3.Vec{vmax.X, vmax.Y, bb.Max.Y}}
	return &s, nil
}

// Revolve3D returns an SDF3 for a solid of revolution.
func Revolve3D(sdf SDF2) (SDF3, error) {
	return RevolveTheta3D(sdf, 0)
}

// Evaluate returns the minimum distance to a solid of revolution.
func (s *SorSDF3) Evaluate(p r3.Vec) float64 {
	x := math.Sqrt(p.X*p.X + p.Y*p.Y)
	a := s.sdf.Evaluate(r2.Vec{x, p.Z})
	b := a
	if s.theta != 0 {
		// combine two vertical planes to give an intersection wedge
		d := s.norm.Dot(r2.Vec{p.X, p.Y})
		if s.theta < pi {
			b = math.Max(-p.Y, d) // intersect
		} else {
			b = math.Min(-p.Y, d) // union
		}
	}
	// return the intersection
	return math.Max(a, b)
}

// BoundingBox returns the bounding box for a solid of revolution.
func (s *SorSDF3) BoundingBox() d3.Box {
	return s.bb
}

// ExtrudeSDF3 extrudes an SDF2 to an SDF3.
type ExtrudeSDF3 struct {
	sdf     SDF2
	height  float64
	extrude ExtrudeFunc
	bb      d3.Box
}

// Extrude3D does a linear extrude on an SDF3.
func Extrude3D(sdf SDF2, height float64) SDF3 {
	s := ExtrudeSDF3{}
	s.sdf = sdf
	s.height = height / 2
	s.extrude = NormalExtrude
	// work out the bounding box
	bb := sdf.BoundingBox()
	s.bb = d3.Box{r3.Vec{bb.Min.X, bb.Min.Y, -s.height}, r3.Vec{bb.Max.X, bb.Max.Y, s.height}}
	return &s
}

// TwistExtrude3D extrudes an SDF2 while rotating by twist radians over the height of the extrusion.
func TwistExtrude3D(sdf SDF2, height, twist float64) SDF3 {
	s := ExtrudeSDF3{}
	s.sdf = sdf
	s.height = height / 2
	s.extrude = TwistExtrude(height, twist)
	// work out the bounding box
	bb := sdf.BoundingBox()
	l := r2.Norm(bb.Max)
	s.bb = d3.Box{r3.Vec{-l, -l, -s.height}, r3.Vec{l, l, s.height}}
	return &s
}

// ScaleExtrude3D extrudes an SDF2 and scales it over the height of the extrusion.
func ScaleExtrude3D(sdf SDF2, height float64, scale r2.Vec) SDF3 {
	s := ExtrudeSDF3{}
	s.sdf = sdf
	s.height = height / 2
	s.extrude = ScaleExtrude(height, scale)
	// work out the bounding box
	bb := sdf.BoundingBox()
	bb = bb.Extend(d2.Box{d2.MulElem(bb.Min, scale), d2.MulElem(bb.Max, scale)})
	s.bb = d3.Box{r3.Vec{bb.Min.X, bb.Min.Y, -s.height}, r3.Vec{bb.Max.X, bb.Max.Y, s.height}}
	return &s
}

// ScaleTwistExtrude3D extrudes an SDF2 and scales and twists it over the height of the extrusion.
func ScaleTwistExtrude3D(sdf SDF2, height, twist float64, scale r2.Vec) SDF3 {
	s := ExtrudeSDF3{}
	s.sdf = sdf
	s.height = height / 2
	s.extrude = ScaleTwistExtrude(height, twist, scale)
	// work out the bounding box
	bb := sdf.BoundingBox()
	bb = bb.Extend(d2.Box{d2.MulElem(bb.Min, scale), d2.MulElem(bb.Max, scale)})
	l := r2.Norm(bb.Max)
	s.bb = d3.Box{r3.Vec{-l, -l, -s.height}, r3.Vec{l, l, s.height}}
	return &s
}

// Evaluate returns the minimum distance to an extrusion.
func (s *ExtrudeSDF3) Evaluate(p r3.Vec) float64 {
	// sdf for the projected 2d surface
	a := s.sdf.Evaluate(s.extrude(p))
	// sdf for the extrusion region: z = [-height, height]
	b := math.Abs(p.Z) - s.height
	// return the intersection
	return math.Max(a, b)
}

// SetExtrude sets the extrusion control function.
func (s *ExtrudeSDF3) SetExtrude(extrude ExtrudeFunc) {
	s.extrude = extrude
}

// BoundingBox returns the bounding box for an extrusion.
func (s *ExtrudeSDF3) BoundingBox() d3.Box {
	return s.bb
}

// Linear extrude an SDF2 with rounded edges.
// Note: The height of the extrusion is adjusted for the rounding.
// The underlying SDF2 shape is not modified.

// ExtrudeRoundedSDF3 extrudes an SDF2 to an SDF3 with rounded edges.
type ExtrudeRoundedSDF3 struct {
	sdf    SDF2
	height float64
	round  float64
	bb     d3.Box
}

// ExtrudeRounded3D extrudes an SDF2 to an SDF3 with rounded edges.
func ExtrudeRounded3D(sdf SDF2, height, round float64) (SDF3, error) {
	if round == 0 {
		// revert to non-rounded case
		return Extrude3D(sdf, height), nil
	}
	if sdf == nil {
		return nil, errors.New("sdf == nil")
	}
	if height <= 0 {
		return nil, errors.New("height <= 0")
	}
	if round < 0 {
		return nil, errors.New("round < 0")
	}
	if height < 2*round {
		return nil, errors.New("height < 2 * round")
	}
	s := ExtrudeRoundedSDF3{
		sdf:    sdf,
		height: (height / 2) - round,
		round:  round,
	}
	// work out the bounding box
	bb := sdf.BoundingBox()
	s.bb = d3.Box{
		Min: r3.Sub(r3.Vec{bb.Min.X, bb.Min.Y, -s.height}, d3.Elem(round)),
		Max: r3.Add(r3.Vec{bb.Max.X, bb.Max.Y, s.height}, d3.Elem(round)),
	}
	return &s, nil
}

// Evaluate returns the minimum distance to a rounded extrusion.
func (s *ExtrudeRoundedSDF3) Evaluate(p r3.Vec) float64 {
	// sdf for the projected 2d surface
	a := s.sdf.Evaluate(r2.Vec{p.X, p.Y})
	b := math.Abs(p.Z) - s.height
	var d float64
	if b > 0 {
		// outside the object Z extent
		if a < 0 {
			// inside the boundary
			d = b
		} else {
			// outside the boundary
			d = math.Sqrt((a * a) + (b * b))
		}
	} else {
		// within the object Z extent
		if a < 0 {
			// inside the boundary
			d = math.Max(a, b)
		} else {
			// outside the boundary
			d = a
		}
	}
	return d - s.round
}

// BoundingBox returns the bounding box for a rounded extrusion.
func (s *ExtrudeRoundedSDF3) BoundingBox() d3.Box {
	return s.bb
}

// Extrude/Loft (with rounded edges)
// Blend between sdf0 and sdf1 as we move from bottom to top.

// LoftSDF3 is an extrusion between two SDF2s.
type LoftSDF3 struct {
	sdf0, sdf1 SDF2
	height     float64
	round      float64
	bb         d3.Box
}

// Loft3D extrudes an SDF3 that transitions between two SDF2 shapes.
func Loft3D(sdf0, sdf1 SDF2, height, round float64) (SDF3, error) {
	if sdf0 == nil {
		return nil, errors.New("sdf0 == nil")
	}
	if sdf1 == nil {
		return nil, errors.New("sdf1 == nil")
	}
	if height <= 0 {
		return nil, errors.New("height <= 0")
	}
	if round < 0 {
		return nil, errors.New("round < 0")
	}
	if height < 2*round {
		return nil, errors.New("height < 2 * round")
	}
	s := LoftSDF3{
		sdf0:   sdf0,
		sdf1:   sdf1,
		height: (height / 2) - round,
		round:  round,
	}
	// work out the bounding box
	bb0 := sdf0.BoundingBox()
	bb1 := sdf1.BoundingBox()
	bb := bb0.Extend(bb1)
	s.bb = d3.Box{
		Min: r3.Sub(r3.Vec{bb.Min.X, bb.Min.Y, -s.height}, d3.Elem(round)),
		Max: r3.Add(r3.Vec{bb.Max.X, bb.Max.Y, s.height}, d3.Elem(round))}
	return &s, nil
}

// Evaluate returns the minimum distance to a loft extrusion.
func (s *LoftSDF3) Evaluate(p r3.Vec) float64 {
	// work out the mix value as a function of height
	k := Clamp((0.5*p.Z/s.height)+0.5, 0, 1)
	// mix the 2D SDFs
	a0 := s.sdf0.Evaluate(r2.Vec{p.X, p.Y})
	a1 := s.sdf1.Evaluate(r2.Vec{p.X, p.Y})
	a := Mix(a0, a1, k)

	b := math.Abs(p.Z) - s.height
	var d float64
	if b > 0 {
		// outside the object Z extent
		if a < 0 {
			// inside the boundary
			d = b
		} else {
			// outside the boundary
			d = math.Sqrt((a * a) + (b * b))
		}
	} else {
		// within the object Z extent
		if a < 0 {
			// inside the boundary
			d = math.Max(a, b)
		} else {
			// outside the boundary
			d = a
		}
	}
	return d - s.round
}

// BoundingBox returns the bounding box for a loft extrusion.
func (s *LoftSDF3) BoundingBox() d3.Box {
	return s.bb
}

// Box (exact distance field)

// BoxSDF3 is a 3d box.
type BoxSDF3 struct {
	size  r3.Vec
	round float64
	bb    d3.Box
}

// Box3D return an SDF3 for a 3d box (rounded corners with round > 0).
func Box3D(size r3.Vec, round float64) (SDF3, error) {
	if d3.LTEZero(size) {
		return nil, ErrMsg("size <= 0")
	}
	if round < 0 {
		return nil, ErrMsg("round < 0")
	}
	size = r3.Scale(0.5, size)
	s := BoxSDF3{
		size:  r3.Sub(size, d3.Elem(round)),
		round: round,
		bb:    d3.Box{Min: r3.Scale(-1, size), Max: size},
	}
	return &s, nil
}

// Evaluate returns the minimum distance to a 3d box.
func (s *BoxSDF3) Evaluate(p r3.Vec) float64 {
	return sdfBox3d(p, s.size) - s.round
}

// BoundingBox returns the bounding box for a 3d box.
func (s *BoxSDF3) BoundingBox() d3.Box {
	return s.bb
}

// Sphere (exact distance field)

// SphereSDF3 is a sphere.
type SphereSDF3 struct {
	radius float64
	bb     d3.Box
}

// Sphere3D return an SDF3 for a sphere.
func Sphere3D(radius float64) (SDF3, error) {
	if radius <= 0 {
		return nil, ErrMsg("radius <= 0")
	}
	d := r3.Vec{radius, radius, radius}
	s := SphereSDF3{
		radius: radius,
		bb:     d3.Box{Min: r3.Scale(-1, d), Max: d},
	}
	return &s, nil
}

// Evaluate returns the minimum distance to a sphere.
func (s *SphereSDF3) Evaluate(p r3.Vec) float64 {
	return r3.Norm(p) - s.radius
}

// BoundingBox returns the bounding box for a sphere.
func (s *SphereSDF3) BoundingBox() d3.Box {
	return s.bb
}

// Cylinder (exact distance field)

// CylinderSDF3 is a cylinder.
type CylinderSDF3 struct {
	height float64
	radius float64
	round  float64
	bb     d3.Box
}

// Cylinder3D return an SDF3 for a cylinder (rounded edges with round > 0).
func Cylinder3D(height, radius, round float64) (SDF3, error) {
	if radius <= 0 {
		return nil, ErrMsg("radius <= 0")
	}
	if round < 0 {
		return nil, ErrMsg("round < 0")
	}
	if round > radius {
		return nil, ErrMsg("round > radius")
	}
	if height < 2.0*round {
		return nil, ErrMsg("height < 2 * round")
	}
	s := CylinderSDF3{}
	s.height = (height / 2) - round
	s.radius = radius - round
	s.round = round
	d := r3.Vec{radius, radius, height / 2}
	s.bb = d3.Box{r3.Scale(-1, d), d}
	return &s, nil
}

// Capsule3D return an SDF3 for a capsule.
func Capsule3D(height, radius float64) (SDF3, error) {
	return Cylinder3D(height, radius, radius)
}

// Evaluate returns the minimum distance to a cylinder.
func (s *CylinderSDF3) Evaluate(p r3.Vec) float64 {
	d := sdfBox2d(r2.Vec{r2.Norm(r2.Vec{p.X, p.Y}), p.Z}, r2.Vec{s.radius, s.height})
	return d - s.round
}

// BoundingBox returns the bounding box for a cylinder.
func (s *CylinderSDF3) BoundingBox() d3.Box {
	return s.bb
}

// Truncated Cone (exact distance field)

// ConeSDF3 is a truncated cone.
type ConeSDF3 struct {
	r0     float64 // base radius
	r1     float64 // top radius
	height float64 // half height
	round  float64 // rounding offset
	u      r2.Vec  // normalized cone slope vector
	n      r2.Vec  // normal to cone slope (points outward)
	l      float64 // length of cone slope
	bb     d3.Box  // bounding box
}

// Cone3D returns the SDF3 for a trucated cone (round > 0 gives rounded edges).
func Cone3D(height, r0, r1, round float64) (SDF3, error) {
	if height <= 0 {
		return nil, ErrMsg("height <= 0")
	}
	if round < 0 {
		return nil, ErrMsg("round < 0")
	}
	if height < 2.0*round {
		return nil, ErrMsg("height < 2 * round")
	}
	s := ConeSDF3{}
	s.height = (height / 2) - round
	s.round = round
	// cone slope vector and normal
	s.u = r2.Unit(r2.Vec{r1, height / 2}.Sub(r2.Vec{r0, -height / 2}))
	s.n = r2.Vec{s.u.Y, -s.u.X}
	// inset the radii for the rounding
	ofs := round / s.n.X
	s.r0 = r0 - (1+s.n.Y)*ofs
	s.r1 = r1 - (1-s.n.Y)*ofs
	// cone slope length
	s.l = r2.Norm(r2.Vec{s.r1, s.height}.Sub(r2.Vec{s.r0, -s.height}))
	// work out the bounding box
	r := math.Max(s.r0+round, s.r1+round)
	s.bb = d3.Box{r3.Vec{-r, -r, -height / 2}, r3.Vec{r, r, height / 2}}
	return &s, nil
}

// Evaluate returns the minimum distance to a trucated cone.
func (s *ConeSDF3) Evaluate(p r3.Vec) float64 {
	// convert to SoR 2d coordinates
	p2 := r2.Vec{math.Hypot(p.X, p.Y), p.Z}
	// is p2 above the cone?
	if p2.Y >= s.height && p2.X <= s.r1 {
		return p2.Y - s.height - s.round
	}
	// is p2 below the cone?
	if p2.Y <= -s.height && p2.X <= s.r0 {
		return -p2.Y - s.height - s.round
	}
	// distance to slope line
	v := p2.Sub(r2.Vec{s.r0, -s.height})
	dSlope := v.Dot(s.n)
	// is p2 inside the cone?
	if dSlope < 0 && math.Abs(p2.Y) < s.height {
		return -math.Min(-dSlope, s.height-math.Abs(p2.Y)) - s.round
	}
	// is p2 closest to the slope line?
	t := v.Dot(s.u)
	if t >= 0 && t <= s.l {
		return dSlope - s.round
	}
	// is p2 closest to the base radius vertex?
	if t < 0 {
		return r2.Norm(v) - s.round
	}
	// p2 is closest to the top radius vertex
	return r2.Norm(p2.Sub(r2.Vec{s.r1, s.height})) - s.round
}

// BoundingBox return the bounding box for the trucated cone..
func (s *ConeSDF3) BoundingBox() d3.Box {
	return s.bb
}

// Transform SDF3 (rotation, translation - distance preserving)

// TransformSDF3 is an SDF3 transformed with a 4x4 transformation matrix.
type TransformSDF3 struct {
	sdf     SDF3
	matrix  m44
	inverse m44
	bb      d3.Box
}

// Transform3D applies a transformation matrix to an SDF3.
func Transform3D(sdf SDF3, matrix m44) SDF3 {
	s := TransformSDF3{}
	s.sdf = sdf
	s.matrix = matrix
	s.inverse = matrix.Inverse()
	s.bb = matrix.MulBox(sdf.BoundingBox())
	return &s
}

// Evaluate returns the minimum distance to a transformed SDF3.
// Distance is *not* preserved with scaling.
func (s *TransformSDF3) Evaluate(p r3.Vec) float64 {
	return s.sdf.Evaluate(s.inverse.MulPosition(p))
}

// BoundingBox returns the bounding box of a transformed SDF3.
func (s *TransformSDF3) BoundingBox() d3.Box {
	return s.bb
}

// Uniform XYZ Scaling of SDF3s (we can work out the distance)

// ScaleUniformSDF3 is an SDF3 scaled uniformly in XYZ directions.
type ScaleUniformSDF3 struct {
	sdf     SDF3
	k, invK float64
	bb      d3.Box
}

// ScaleUniform3D uniformly scales an SDF3 on all axes.
func ScaleUniform3D(sdf SDF3, k float64) SDF3 {
	m := Scale3d(r3.Vec{k, k, k})
	return &ScaleUniformSDF3{
		sdf:  sdf,
		k:    k,
		invK: 1.0 / k,
		bb:   m.MulBox(sdf.BoundingBox()),
	}
}

// Evaluate returns the minimum distance to a uniformly scaled SDF3.
// The distance is correct with scaling.
func (s *ScaleUniformSDF3) Evaluate(p r3.Vec) float64 {
	q := r3.Scale(s.invK, p)
	return s.sdf.Evaluate(q) * s.k
}

// BoundingBox returns the bounding box of a uniformly scaled SDF3.
func (s *ScaleUniformSDF3) BoundingBox() d3.Box {
	return s.bb
}

// UnionSDF3 is a union of SDF3s.
type UnionSDF3 struct {
	sdf []SDF3
	min MinFunc
	bb  d3.Box
}

// Union3D returns the union of multiple SDF3 objects.
func Union3D(sdf ...SDF3) SDF3 {
	if len(sdf) == 0 {
		return nil
	}
	s := UnionSDF3{}
	// strip out any nils
	s.sdf = make([]SDF3, 0, len(sdf))
	for _, x := range sdf {
		if x != nil {
			s.sdf = append(s.sdf, x)
		}
	}
	if len(s.sdf) == 0 {
		return nil
	}
	if len(s.sdf) == 1 {
		// only one sdf - not really a union
		return s.sdf[0]
	}
	// work out the bounding box
	bb := s.sdf[0].BoundingBox()
	for _, x := range s.sdf {
		bb = bb.Extend(x.BoundingBox())
	}
	s.bb = bb
	s.min = math.Min
	return &s
}

// Evaluate returns the minimum distance to an SDF3 union.
func (s *UnionSDF3) Evaluate(p r3.Vec) float64 {
	var d float64
	for i, x := range s.sdf {
		if i == 0 {
			d = x.Evaluate(p)
		} else {
			d = s.min(d, x.Evaluate(p))
		}
	}
	return d
}

// SetMin sets the minimum function to control blending.
func (s *UnionSDF3) SetMin(min MinFunc) {
	s.min = min
}

// BoundingBox returns the bounding box of an SDF3 union.
func (s *UnionSDF3) BoundingBox() d3.Box {
	return s.bb
}

// DifferenceSDF3 is the difference of two SDF3s, s0 - s1.
type DifferenceSDF3 struct {
	s0  SDF3
	s1  SDF3
	max MaxFunc
	bb  d3.Box
}

// Difference3D returns the difference of two SDF3s, s0 - s1.
func Difference3D(s0, s1 SDF3) SDF3 {
	if s1 == nil {
		return s0
	}
	if s0 == nil {
		return nil
	}
	s := DifferenceSDF3{}
	s.s0 = s0
	s.s1 = s1
	s.max = math.Max
	s.bb = s0.BoundingBox()
	return &s
}

// Evaluate returns the minimum distance to the SDF3 difference.
func (s *DifferenceSDF3) Evaluate(p r3.Vec) float64 {
	return s.max(s.s0.Evaluate(p), -s.s1.Evaluate(p))
}

// SetMax sets the maximum function to control blending.
func (s *DifferenceSDF3) SetMax(max MaxFunc) {
	s.max = max
}

// BoundingBox returns the bounding box of the SDF3 difference.
func (s *DifferenceSDF3) BoundingBox() d3.Box {
	return s.bb
}

// ElongateSDF3 is the elongation of an SDF3.
type ElongateSDF3 struct {
	sdf    SDF3   // the sdf being elongated
	hp, hn r3.Vec // positive/negative elongation vector
	bb     d3.Box // bounding box
}

// Elongate3D returns the elongation of an SDF3.
func Elongate3D(sdf SDF3, h r3.Vec) SDF3 {
	h = d3.AbsElem(h)
	s := ElongateSDF3{
		sdf: sdf,
		hp:  r3.Scale(0.5, h),
		hn:  r3.Scale(-0.5, h),
	}
	// bounding box
	bb := sdf.BoundingBox()
	bb0 := bb.Translate(s.hp)
	bb1 := bb.Translate(s.hn)
	s.bb = bb0.Extend(bb1)
	return &s
}

// Evaluate returns the minimum distance to a elongated SDF2.
func (s *ElongateSDF3) Evaluate(p r3.Vec) float64 {
	q := p.Sub(d3.Clamp(p, s.hn, s.hp))
	return s.sdf.Evaluate(q)
}

// BoundingBox returns the bounding box of an elongated SDF3.
func (s *ElongateSDF3) BoundingBox() d3.Box {
	return s.bb
}

// IntersectionSDF3 is the intersection of two SDF3s.
type IntersectionSDF3 struct {
	s0  SDF3
	s1  SDF3
	max MaxFunc
	bb  d3.Box
}

// Intersect3D returns the intersection of two SDF3s.
func Intersect3D(s0, s1 SDF3) SDF3 {
	if s0 == nil || s1 == nil {
		return nil
	}
	s := IntersectionSDF3{}
	s.s0 = s0
	s.s1 = s1
	s.max = math.Max
	// TODO fix bounding box
	s.bb = s0.BoundingBox()
	return &s
}

// Evaluate returns the minimum distance to the SDF3 intersection.
func (s *IntersectionSDF3) Evaluate(p r3.Vec) float64 {
	return s.max(s.s0.Evaluate(p), s.s1.Evaluate(p))
}

// SetMax sets the maximum function to control blending.
func (s *IntersectionSDF3) SetMax(max MaxFunc) {
	s.max = max
}

// BoundingBox returns the bounding box of an SDF3 intersection.
func (s *IntersectionSDF3) BoundingBox() d3.Box {
	return s.bb
}

// CutSDF3 makes a planar cut through an SDF3.
type CutSDF3 struct {
	sdf SDF3
	a   r3.Vec // point on plane
	n   r3.Vec // normal to plane
	bb  d3.Box // bounding box
}

// Cut3D cuts an SDF3 along a plane passing through a with normal n.
// The SDF3 on the same side as the normal remains.
func Cut3D(sdf SDF3, a, n r3.Vec) SDF3 {
	s := CutSDF3{}
	s.sdf = sdf
	s.a = a
	s.n = r3.Scale(-1, r3.Unit(n))
	// TODO - cut the bounding box
	s.bb = sdf.BoundingBox()
	return &s
}

// Evaluate returns the minimum distance to the cut SDF3.
func (s *CutSDF3) Evaluate(p r3.Vec) float64 {
	return math.Max(p.Sub(s.a).Dot(s.n), s.sdf.Evaluate(p))
}

// BoundingBox returns the bounding box of the cut SDF3.
func (s *CutSDF3) BoundingBox() d3.Box {
	return s.bb
}

// ArraySDF3 stores an XYZ array of a given SDF3
type ArraySDF3 struct {
	sdf  SDF3
	num  V3i
	step r3.Vec
	min  MinFunc
	bb   d3.Box
}

// Array3D returns an XYZ array of a given SDF3
func Array3D(sdf SDF3, num V3i, step r3.Vec) SDF3 {
	// check the number of steps
	if num[0] <= 0 || num[1] <= 0 || num[2] <= 0 {
		return nil
	}
	s := ArraySDF3{}
	s.sdf = sdf
	s.num = num
	s.step = step
	s.min = math.Min
	// work out the bounding box
	bb0 := sdf.BoundingBox()
	bb1 := bb0.Translate(d3.MulElem(step, num.SubScalar(1).ToV3()))
	s.bb = bb0.Extend(bb1)
	return &s
}

// SetMin sets the minimum function to control blending.
func (s *ArraySDF3) SetMin(min MinFunc) {
	s.min = min
}

// Evaluate returns the minimum distance to an XYZ SDF3 array.
func (s *ArraySDF3) Evaluate(p r3.Vec) float64 {
	d := math.MaxFloat64
	for j := 0; j < s.num[0]; j++ {
		for k := 0; k < s.num[1]; k++ {
			for l := 0; l < s.num[2]; l++ {
				x := p.Sub(r3.Vec{float64(j) * s.step.X, float64(k) * s.step.Y, float64(l) * s.step.Z})
				d = s.min(d, s.sdf.Evaluate(x))
			}
		}
	}
	return d
}

// BoundingBox returns the bounding box of an XYZ SDF3 array.
func (s *ArraySDF3) BoundingBox() d3.Box {
	return s.bb
}

// RotateUnionSDF3 creates a union of SDF3s rotated about the z-axis.
type RotateUnionSDF3 struct {
	sdf  SDF3
	num  int
	step m44
	min  MinFunc
	bb   d3.Box
}

// RotateUnion3D creates a union of SDF3s rotated about the z-axis.
func RotateUnion3D(sdf SDF3, num int, step m44) SDF3 {
	// check the number of steps
	if num <= 0 {
		return nil
	}
	s := RotateUnionSDF3{}
	s.sdf = sdf
	s.num = num
	s.step = step.Inverse()
	s.min = math.Min
	// work out the bounding box
	v := sdf.BoundingBox().Vertices()
	bbMin := v[0]
	bbMax := v[0]
	for i := 0; i < s.num; i++ {
		bbMin = d3.MinElem(bbMin, v.Min())
		bbMax = d3.MaxElem(bbMax, v.Max())
		mulVertices3(v, step)
		// v.MulVertices(step)
	}
	s.bb = d3.Box{bbMin, bbMax}
	return &s
}

// Evaluate returns the minimum distance to a rotate/union object.
func (s *RotateUnionSDF3) Evaluate(p r3.Vec) float64 {
	d := math.MaxFloat64
	rot := Identity3d()
	for i := 0; i < s.num; i++ {
		x := rot.MulPosition(p)
		d = s.min(d, s.sdf.Evaluate(x))
		rot = rot.Mul(s.step)
	}
	return d
}

// SetMin sets the minimum function to control blending.
func (s *RotateUnionSDF3) SetMin(min MinFunc) {
	s.min = min
}

// BoundingBox returns the bounding box of a rotate/union object.
func (s *RotateUnionSDF3) BoundingBox() d3.Box {
	return s.bb
}

// RotateCopySDF3 rotates and creates N copies of an SDF3 about the z-axis.
type RotateCopySDF3 struct {
	sdf   SDF3
	theta float64
	bb    d3.Box
}

// RotateCopy3D rotates and creates N copies of an SDF3 about the z-axis.
func RotateCopy3D(
	sdf SDF3, // SDF3 to rotate and copy
	num int, // number of copies
) SDF3 {
	// check the number of steps
	if num <= 0 {
		return nil
	}
	s := RotateCopySDF3{}
	s.sdf = sdf
	s.theta = tau / float64(num)
	// work out the bounding box
	bb := sdf.BoundingBox()
	zmax := bb.Max.Z
	zmin := bb.Min.Z
	rmax := 0.0
	// find the bounding box vertex with the greatest distance from the z-axis
	// TODO - revisit - should go by real vertices
	for _, v := range bb.Vertices() {
		l := math.Hypot(v.X, v.Y)
		if l > rmax {
			rmax = l
		}
	}
	s.bb = d3.Box{r3.Vec{-rmax, -rmax, zmin}, r3.Vec{rmax, rmax, zmax}}
	return &s
}

// Evaluate returns the minimum distance to a rotate/copy SDF3.
func (s *RotateCopySDF3) Evaluate(p r3.Vec) float64 {
	// Map p to a point in the first copy sector.
	p2 := r2.Vec{p.X, p.Y}
	p2 = d2.PolarToXY(r2.Norm(p2), SawTooth(math.Atan2(p2.Y, p2.X), s.theta))
	return s.sdf.Evaluate(r3.Vec{p2.X, p2.Y, p.Z})
}

// BoundingBox returns the bounding box of a rotate/copy SDF3.
func (s *RotateCopySDF3) BoundingBox() d3.Box {
	return s.bb
}

/* WIP

// Connector3 defines a 3d connection point.
type Connector3 struct {
	Name     string
	Position r3.Vec
	Vector   r3.Vec
	Angle    float64
}

// ConnectedSDF3 is an SDF3 with connection points defined.
type ConnectedSDF3 struct {
	sdf        SDF3
	connectors []Connector3
}

// AddConnector adds connection points to an SDF3.
func AddConnector(sdf SDF3, connectors ...Connector3) SDF3 {
	// is the sdf already connected?
	if s, ok := sdf.(*ConnectedSDF3); ok {
		// append connection points
		s.connectors = append(s.connectors, connectors...)
		return s
	}
	// return a new connected sdf
	return &ConnectedSDF3{
		sdf:        sdf,
		connectors: connectors,
	}
}

// Evaluate returns the minimum distance to a connected SDF3.
func (s *ConnectedSDF3) Evaluate(p r3.Vec) float64 {
	return s.sdf.Evaluate(p)
}

// BoundingBox returns the bounding box of a connected SDF3.
func (s *ConnectedSDF3) BoundingBox() d3.Box {
	return s.sdf.BoundingBox()
}

*/

// OffsetSDF3 offsets the distance function of an existing SDF3.
type OffsetSDF3 struct {
	sdf    SDF3    // the underlying SDF
	offset float64 // the distance the SDF is offset by
	bb     d3.Box  // bounding box
}

// Offset3D returns an SDF3 that offsets the distance function of another SDF3.
func Offset3D(sdf SDF3, offset float64) SDF3 {
	s := OffsetSDF3{
		sdf:    sdf,
		offset: offset,
	}
	// bounding box
	bb := sdf.BoundingBox()
	s.bb = d3.NewBox(bb.Center(), r3.Add(bb.Size(), d3.Elem(2*offset)))
	return &s
}

// Evaluate returns the minimum distance to an offset SDF3.
func (s *OffsetSDF3) Evaluate(p r3.Vec) float64 {
	return s.sdf.Evaluate(p) - s.offset
}

// BoundingBox returns the bounding box of an offset SDF3.
func (s *OffsetSDF3) BoundingBox() d3.Box {
	return s.bb
}

// ShellSDF3 shells the surface of an existing SDF3.
type ShellSDF3 struct {
	sdf   SDF3    // parent sdf3
	delta float64 // half shell thickness
	bb    d3.Box  // bounding box
}

// Shell3D returns an SDF3 that shells the surface of an existing SDF3.
func Shell3D(sdf SDF3, thickness float64) (SDF3, error) {
	if thickness <= 0 {
		return nil, ErrMsg("thickness <= 0")
	}
	return &ShellSDF3{
		sdf:   sdf,
		delta: 0.5 * thickness,
		bb:    sdf.BoundingBox().Enlarge(r3.Vec{thickness, thickness, thickness}),
	}, nil
}

// Evaluate returns the minimum distance to a shelled SDF3.
func (s *ShellSDF3) Evaluate(p r3.Vec) float64 {
	return math.Abs(s.sdf.Evaluate(p)) - s.delta
}

// BoundingBox returns the bounding box of a shelled SDF3.
func (s *ShellSDF3) BoundingBox() d3.Box {
	return s.bb
}

// LineOf3D returns a union of 3D objects positioned along a line from p0 to p1.
func LineOf3D(s SDF3, p0, p1 r3.Vec, pattern string) SDF3 {
	var objects []SDF3
	if pattern != "" {
		x := p0
		dx := r3.Scale(1/float64(len(pattern)), r3.Sub(p1, p0))
		// dx := p1.Sub(p0).DivScalar(float64(len(pattern))) //TODO VERIFY
		for _, c := range pattern {
			if c == 'x' {
				objects = append(objects, Transform3D(s, Translate3d(x)))
			}
			x = x.Add(dx)
		}
	}
	return Union3D(objects...)
}

// Multi3D creates a union of an SDF3 at translated positions.
func Multi3D(s SDF3, positions d3.Set) SDF3 {
	if (s == nil) || (len(positions) == 0) {
		return nil
	}
	objects := make([]SDF3, len(positions))
	for i, p := range positions {
		objects[i] = Transform3D(s, Translate3d(p))
	}
	return Union3D(objects...)
}

// Orient3D creates a union of an SDF3 at oriented directions.
func Orient3D(s SDF3, base r3.Vec, directions d3.Set) SDF3 {
	if (s == nil) || (len(directions) == 0) {
		return nil
	}
	objects := make([]SDF3, len(directions))
	for i, d := range directions {
		objects[i] = Transform3D(s, rotateToVec(base, d))
	}
	return Union3D(objects...)
}
