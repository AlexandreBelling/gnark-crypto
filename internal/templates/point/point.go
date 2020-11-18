package point

// Types ...
const Types = `

{{ $TAffine := print (toUpper .PointName) "Affine" }}
{{ $TJacobian := print (toUpper .PointName) "Jac" }}
{{ $TJacobianExtended := print (toLower .PointName) "JacExtended" }}
{{ $TProjective := print (toLower .PointName) "Proj" }}


`

// Point ...
const Point = `

import (
	"io"
	"math/big"
	"encoding/binary"
	"errors"

	"github.com/consensys/gurvy/{{ toLower .Name}}/fp"
	"github.com/consensys/gurvy/{{ toLower .Name}}/fr"
	"github.com/consensys/gurvy/utils"
	"github.com/consensys/gurvy/utils/parallel"
	{{if eq .CoordType "fptower.E2"}}"github.com/consensys/gurvy/{{ toLower .Name}}/internal/fptower"{{end}}
)

// {{ $TAffine }} point in affine coordinates
type {{ $TAffine }} struct {
	X, Y {{.CoordType}}
}

// {{ $TJacobian }} is a point with {{.CoordType}} coordinates
type {{ $TJacobian }} struct {
	X, Y, Z {{.CoordType}}
}

//  {{ $TJacobianExtended }} parameterized jacobian coordinates (x=X/ZZ, y=Y/ZZZ, ZZ**3=ZZZ**2)
type {{ $TJacobianExtended }} struct {
	X, Y, ZZ, ZZZ {{.CoordType}}
}

// {{ $TProjective }} point in projective coordinates
type {{ $TProjective }} struct {
	x, y, z {{.CoordType}}
}



// -------------------------------------------------------------------------------------------------
// Affine 


// ScalarMultiplication computes and returns p = a*s
func (p *{{ $TAffine }}) ScalarMultiplication(a *{{ $TAffine }}, s *big.Int) *{{ $TAffine }} {
	var _p {{ $TJacobian }}
	_p.FromAffine(a)
	_p.mulGLV(&_p, s)
	p.FromJacobian(&_p)
	return p
}



// Equal tests if two points (in Affine coordinates) are equal
func (p *{{ $TAffine }}) Equal(a *{{ $TAffine }}) bool {
	return p.X.Equal(&a.X) && p.Y.Equal(&a.Y)
}


// Neg computes -G
func (p *{{ $TAffine }}) Neg(a *{{ $TAffine }}) *{{ $TAffine }} {
	p.X = a.X
	p.Y.Neg(&a.Y)
	return p
}




// FromJacobian rescale a point in Jacobian coord in z=1 plane
func (p *{{ $TAffine }}) FromJacobian(p1 *{{ $TJacobian }}) *{{ $TAffine }} {

	var a, b {{.CoordType}}

	if p1.Z.IsZero() {
		p.X.SetZero()
		p.Y.SetZero()
		return p
	}

	a.Inverse(&p1.Z)
	b.Square(&a)
	p.X.Mul(&p1.X, &b)
	p.Y.Mul(&p1.Y, &b).Mul(&p.Y, &a)

	return p
}



func (p *{{ $TAffine }}) String() string {
	var x, y {{.CoordType}}
	x.Set(&p.X)
	y.Set(&p.Y)
	return "E([" + x.String() + "," + y.String() + "]),"
}

// IsInfinity checks if the point is infinity (in affine, it's encoded as (0,0))
func (p *{{ $TAffine }}) IsInfinity() bool {
	return p.X.IsZero() && p.Y.IsZero()
}

// IsOnCurve returns true if p in on the curve
func (p *{{ $TAffine }}) IsOnCurve() bool {
	var point {{ $TJacobian }}
	point.FromAffine(p)
	return point.IsOnCurve() // call this function to handle infinity point
}

// IsInSubGroup returns true if p is in the correct subgroup, false otherwise
func (p *{{ $TAffine }}) IsInSubGroup() bool {
	var _p {{ $TJacobian }}
	_p.FromAffine(p)
	return _p.IsOnCurve() && _p.IsInSubGroup()
}


// -------------------------------------------------------------------------------------------------
// Jacobian 

// Set set p to the provided point
func (p *{{ $TJacobian }}) Set(a *{{ $TJacobian }}) *{{ $TJacobian }} {
	p.X, p.Y, p.Z = a.X, a.Y, a.Z
	return p
}

// Equal tests if two points (in Jacobian coordinates) are equal
func (p *{{ $TJacobian }}) Equal(a *{{ $TJacobian }}) bool {

	if p.Z.IsZero() && a.Z.IsZero() {
		return true
	}
	_p := {{ $TAffine }}{}
	_p.FromJacobian(p)

	_a := {{ $TAffine }}{}
	_a.FromJacobian(a)

	return _p.X.Equal(&_a.X) && _p.Y.Equal(&_a.Y)
}

// Neg computes -G
func (p *{{ $TJacobian }}) Neg(a *{{ $TJacobian }}) *{{ $TJacobian }} {
	*p = *a
	p.Y.Neg(&a.Y)
	return p
}


// SubAssign substracts two points on the curve
func (p *{{ $TJacobian }}) SubAssign(a *{{ $TJacobian }}) *{{ $TJacobian }} {
	var tmp {{ $TJacobian }}
	tmp.Set(a)
	tmp.Y.Neg(&tmp.Y)
	p.AddAssign(&tmp)
	return p
}


// AddAssign point addition in montgomery form
// https://hyperelliptic.org/EFD/{{ toLower .PointName }}p/auto-shortw-jacobian-3.html#addition-add-2007-bl
func (p *{{ $TJacobian }}) AddAssign(a *{{ $TJacobian }}) *{{ $TJacobian }} {

	// p is infinity, return a
	if p.Z.IsZero() {
		p.Set(a)
		return p
	}

	// a is infinity, return p
	if a.Z.IsZero() {
		return p
	}

	var Z1Z1, Z2Z2, U1, U2, S1, S2, H, I, J, r, V {{.CoordType}}
	Z1Z1.Square(&a.Z)
	Z2Z2.Square(&p.Z)
	U1.Mul(&a.X, &Z2Z2)
	U2.Mul(&p.X, &Z1Z1)
	S1.Mul(&a.Y, &p.Z).
		Mul(&S1, &Z2Z2)
	S2.Mul(&p.Y, &a.Z).
		Mul(&S2, &Z1Z1)

	// if p == a, we double instead
	if U1.Equal(&U2) && S1.Equal(&S2) {
		return p.DoubleAssign()
	}

	H.Sub(&U2, &U1)
	I.Double(&H).
		Square(&I)
	J.Mul(&H, &I)
	r.Sub(&S2, &S1).Double(&r)
	V.Mul(&U1, &I)
	p.X.Square(&r).
		Sub(&p.X, &J).
		Sub(&p.X, &V).
		Sub(&p.X, &V)
	p.Y.Sub(&V, &p.X).
		Mul(&p.Y, &r)
	S1.Mul(&S1, &J).Double(&S1)
	p.Y.Sub(&p.Y, &S1)
	p.Z.Add(&p.Z, &a.Z)
	p.Z.Square(&p.Z).
		Sub(&p.Z, &Z1Z1).
		Sub(&p.Z, &Z2Z2).
		Mul(&p.Z, &H)

	return p
}

// AddMixed point addition
// http://www.hyperelliptic.org/EFD/{{ toLower .PointName }}p/auto-shortw-jacobian-0.html#addition-madd-2007-bl
func (p *{{ $TJacobian }}) AddMixed(a *{{ $TAffine }}) *{{ $TJacobian }} {

	//if a is infinity return p
	if a.X.IsZero() && a.Y.IsZero() {
		return p
	}
	// p is infinity, return a
	if p.Z.IsZero() {
		p.X = a.X
		p.Y = a.Y
		p.Z.SetOne()
		return p
	}

	// get some Element from our pool
	var Z1Z1, U2, S2, H, HH, I, J, r, V {{.CoordType}}
	Z1Z1.Square(&p.Z)
	U2.Mul(&a.X, &Z1Z1)
	S2.Mul(&a.Y, &p.Z).
		Mul(&S2, &Z1Z1)

	// if p == a, we double instead
	if U2.Equal(&p.X) && S2.Equal(&p.Y) {
		return p.DoubleAssign()
	}

	H.Sub(&U2, &p.X)
	HH.Square(&H)
	I.Double(&HH).Double(&I)
	J.Mul(&H, &I)
	r.Sub(&S2, &p.Y).Double(&r)
	V.Mul(&p.X, &I)
	p.X.Square(&r).
		Sub(&p.X, &J).
		Sub(&p.X, &V).
		Sub(&p.X, &V)
	J.Mul(&J, &p.Y).Double(&J)
	p.Y.Sub(&V, &p.X).
		Mul(&p.Y, &r)
	p.Y.Sub(&p.Y, &J)
	p.Z.Add(&p.Z, &H)
	p.Z.Square(&p.Z).
		Sub(&p.Z, &Z1Z1).
		Sub(&p.Z, &HH)

	return p
}

// Double doubles a point in Jacobian coordinates
// https://hyperelliptic.org/EFD/{{ toLower .PointName }}p/auto-shortw-jacobian-3.html#doubling-dbl-2007-bl
func (p *{{ $TJacobian }}) Double(q *{{ $TJacobian }}) *{{ $TJacobian }} {
	p.Set(q)
	p.DoubleAssign()
	return p
}

// DoubleAssign doubles a point in Jacobian coordinates
// https://hyperelliptic.org/EFD/{{ toLower .PointName }}p/auto-shortw-jacobian-3.html#doubling-dbl-2007-bl
func (p *{{ $TJacobian }}) DoubleAssign() *{{ $TJacobian }} {

	// get some Element from our pool
	var XX, YY, YYYY, ZZ, S, M, T {{.CoordType}}

	XX.Square(&p.X)
	YY.Square(&p.Y)
	YYYY.Square(&YY)
	ZZ.Square(&p.Z)
	S.Add(&p.X, &YY)
	S.Square(&S).
		Sub(&S, &XX).
		Sub(&S, &YYYY).
		Double(&S)
	M.Double(&XX).Add(&M, &XX)
	p.Z.Add(&p.Z, &p.Y).
		Square(&p.Z).
		Sub(&p.Z, &YY).
		Sub(&p.Z, &ZZ)
	T.Square(&M)
	p.X = T
	T.Double(&S)
	p.X.Sub(&p.X, &T)
	p.Y.Sub(&S, &p.X).
		Mul(&p.Y, &M)
	YYYY.Double(&YYYY).Double(&YYYY).Double(&YYYY)
	p.Y.Sub(&p.Y, &YYYY)

	return p
}


// ScalarMultiplication computes and returns p = a*s
// {{- if .GLV}} see https://www.iacr.org/archive/crypto2001/21390189.pdf {{- else }} using 2-bits windowed exponentiation {{- end }}
func (p *{{ $TJacobian }}) ScalarMultiplication(a *{{ $TJacobian }}, s *big.Int) *{{ $TJacobian }} {
	{{- if .GLV}}
		return p.mulGLV(a, s)
	{{- else }}
		return p.mulWindowed(a, s)
	{{- end }}
}

func (p *{{ $TJacobian }}) String() string {
	if p.Z.IsZero() {
		return "O"
	}
	_p := {{ $TAffine }}{}
	_p.FromJacobian(p)
	return "E([" + _p.X.String() + "," + _p.Y.String() + "]),"
}

// FromAffine sets p = Q, p in Jacboian, Q in affine
func (p *{{ $TJacobian }}) FromAffine(Q *{{ $TAffine }}) *{{ $TJacobian }} {
	if Q.X.IsZero() && Q.Y.IsZero() {
		p.Z.SetZero()
		p.X.SetOne()
		p.Y.SetOne()
		return p
	}
	p.Z.SetOne()
	p.X.Set(&Q.X)
	p.Y.Set(&Q.Y)
	return p
}


// IsOnCurve returns true if p in on the curve
func (p *{{ $TJacobian }}) IsOnCurve() bool {
	var left, right, tmp  {{.CoordType}}
	left.Square(&p.Y)
	right.Square(&p.X).Mul(&right, &p.X)
	tmp.Square(&p.Z).
		Square(&tmp).
		Mul(&tmp, &p.Z).
		Mul(&tmp, &p.Z).
		{{- if eq .PointName "g1"}}
			Mul(&tmp, &bCurveCoeff)
		{{- else}}
			Mul(&tmp, &bTwistCurveCoeff)
		{{- end}}
	right.Add(&right, &tmp)
	return left.Equal(&right)
}



{{if eq .Name "bn256" }}
	{{if eq .PointName "g1"}}
		// IsInSubGroup returns true if p is on the r-torsion, false otherwise.
		// For bn curves, the r-torsion in E(Fp) is the full group, so we just check that
		// the point is on the curve.
		func (p *{{ $TJacobian }}) IsInSubGroup() bool {

			return p.IsOnCurve()

		}
	{{else if eq .PointName "g2"}}
		// IsInSubGroup returns true if p is on the r-torsion, false otherwise.
		// Z[r,0]+Z[-lambda{{ $TAffine }}, 1] is the kernel
		// of (u,v)->u+lambda{{ $TAffine }}v mod r. Expressing r, lambda{{ $TAffine }} as
		// polynomials in x, a short vector of this Zmodule is
		// (4x+2), (-12x**2+4*x). So we check that (4x+2)p+(-12x**2+4*x)phi(p)
		// is the infinity.
		func (p *{{ $TJacobian }}) IsInSubGroup() bool {

			var res, xphip, phip {{ $TJacobian }}
			phip.phi(p)
			xphip.ScalarMultiplication(&phip, &xGen)           // x*phi(p)
			res.Double(&xphip).AddAssign(&xphip)               // 3x*phi(p)
			res.AddAssign(&phip).SubAssign(p)                  // 3x*phi(p)+phi(p)-p
			res.Double(&res).ScalarMultiplication(&res, &xGen) // 6x**2*phi(p)+2x*phi(p)-2x*p
			res.SubAssign(p).Double(&res)                      // 12x**2*phi(p)+4x*phi(p)-4x*p-2p

			return res.IsOnCurve() && res.Z.IsZero()

		}
	{{end}}
{{else if eq .Name "bw761" }}
	// IsInSubGroup returns true if p is on the r-torsion, false otherwise.
	// Z[r,0]+Z[-lambda{{ $TAffine }}, 1] is the kernel
	// of (u,v)->u+lambda{{ $TAffine }}v mod r. Expressing r, lambda{{ $TAffine }} as
	// polynomials in x, a short vector of this Zmodule is
	// (x+1), (x**3-x**2+1). So we check that (x+1)p+(x**3-x**2+1)*phi(p)
	// is the infinity.
	func (p *{{ $TJacobian }}) IsInSubGroup() bool {

		var res, phip {{ $TJacobian }}
		phip.phi(p)
		res.ScalarMultiplication(&phip, &xGen).
			SubAssign(&phip).
			ScalarMultiplication(&res, &xGen).
			ScalarMultiplication(&res, &xGen).
			AddAssign(&phip)

		phip.ScalarMultiplication(p, &xGen).AddAssign(p).AddAssign(&res)

		return phip.IsOnCurve() && phip.Z.IsZero()

	}
{{else}}
	// IsInSubGroup returns true if p is on the r-torsion, false otherwise.
	// Z[r,0]+Z[-lambda{{ $TAffine }}, 1] is the kernel
	// of (u,v)->u+lambda{{ $TAffine }}v mod r. Expressing r, lambda{{ $TAffine }} as
	// polynomials in x, a short vector of this Zmodule is
	// 1, x**2. So we check that p+x**2*phi(p)
	// is the infinity.
	func (p *{{ $TJacobian }}) IsInSubGroup() bool {

		var res {{ $TJacobian }}
		res.phi(p).
			ScalarMultiplication(&res, &xGen).
			ScalarMultiplication(&res, &xGen).
			AddAssign(p)

		return res.IsOnCurve() && res.Z.IsZero()

	}
{{end}}


// mulWindowed 2-bits windowed exponentiation
func (p *{{ $TJacobian }}) mulWindowed(a *{{ $TJacobian }}, s *big.Int) *{{ $TJacobian }} {

	var res {{ $TJacobian }}
	var ops [3]{{ $TJacobian }}

	res.Set(&{{ toLower .PointName}}Infinity)
	ops[0].Set(a)
	ops[1].Double(&ops[0])
	ops[2].Set(&ops[0]).AddAssign(&ops[1])

	b := s.Bytes()
	for i := range b {
		w := b[i]
		mask := byte(0xc0)
		for j := 0; j < 4; j++ {
			res.DoubleAssign().DoubleAssign()
			c := (w & mask) >> (6 - 2*j)
			if c != 0 {
				res.AddAssign(&ops[c-1])
			}
			mask = mask >> 2
		}
	}
	p.Set(&res)

	return p

}

{{ if eq .CoordType "fptower.E2" }}
	// psi(p) = u o frob o u**-1 where u:E'->E iso from the twist to E
	func (p *{{ $TJacobian }}) psi(a *{{ $TJacobian }}) *{{ $TJacobian }} {
		p.Set(a)
		p.X.Conjugate(&p.X).Mul(&p.X, &endo.u)
		p.Y.Conjugate(&p.Y).Mul(&p.Y, &endo.v)
		p.Z.Conjugate(&p.Z)
		return p
	}
{{ end }}

{{ if .GLV}}

// phi assigns p to phi(a) where phi: (x,y)->(ux,y), and returns p
func (p *{{ $TJacobian }}) phi(a *{{ $TJacobian }}) *{{ $TJacobian }} {
	p.Set(a)
	{{if eq .CoordType "fptower.E2"}}
		p.X.MulByElement(&p.X, &thirdRootOne{{toUpper .PointName}})
	{{else}}
		p.X.Mul(&p.X, &thirdRootOne{{toUpper .PointName}})
	{{end}}
	return p
}

// mulGLV performs scalar multiplication using GLV
// see https://www.iacr.org/archive/crypto2001/21390189.pdf
func (p *{{ $TJacobian }}) mulGLV(a *{{ $TJacobian }}, s *big.Int) *{{ $TJacobian }} {

	var table [15]{{ $TJacobian }}
	var zero big.Int
	var res {{ $TJacobian }}
	var k1, k2 fr.Element

	res.Set(&{{ toLower .PointName}}Infinity)

	// table[b3b2b1b0-1] = b3b2*phi(a) + b1b0*a
	table[0].Set(a)
	table[3].phi(a)

	// split the scalar, modifies +-a, phi(a) accordingly
	k := utils.SplitScalar(s, &glvBasis)

	if k[0].Cmp(&zero) == -1 {
		k[0].Neg(&k[0])
		table[0].Neg(&table[0])
	}
	if k[1].Cmp(&zero) == -1 {
		k[1].Neg(&k[1])
		table[3].Neg(&table[3])
	}

	// precompute table (2 bits sliding window)
	// table[b3b2b1b0-1] = b3b2*phi(a) + b1b0*a if b3b2b1b0 != 0
	table[1].Double(&table[0])
	table[2].Set(&table[1]).AddAssign(&table[0])
	table[4].Set(&table[3]).AddAssign(&table[0])
	table[5].Set(&table[3]).AddAssign(&table[1])
	table[6].Set(&table[3]).AddAssign(&table[2])
	table[7].Double(&table[3])
	table[8].Set(&table[7]).AddAssign(&table[0])
	table[9].Set(&table[7]).AddAssign(&table[1])
	table[10].Set(&table[7]).AddAssign(&table[2])
	table[11].Set(&table[7]).AddAssign(&table[3])
	table[12].Set(&table[11]).AddAssign(&table[0])
	table[13].Set(&table[11]).AddAssign(&table[1])
	table[14].Set(&table[11]).AddAssign(&table[2])

	// bounds on the lattice base vectors guarantee that k1, k2 are len(r)/2 bits long max
	k1.SetBigInt(&k[0]).FromMont()
	k2.SetBigInt(&k[1]).FromMont()

	// loop starts from len(k1)/2 due to the bounds
	for i := len(k1)/2 - 1; i >= 0; i-- {
		mask := uint64(3) << 62
		for j := 0; j < 32; j++ {
			res.Double(&res).Double(&res)
			b1 := (k1[i] & mask) >> (62 - 2*j)
			b2 := (k2[i] & mask) >> (62 - 2*j)
			if b1|b2 != 0 {
				s := (b2<<2 | b1)
				res.AddAssign(&table[s-1])
			}
			mask = mask >> 2
		}
	}

	p.Set(&res)
	return p
}


{{ end }}

// -------------------------------------------------------------------------------------------------
// Jacobian extended 




// -------------------------------------------------------------------------------------------------
// Projective

// FromJacobian converts a point from Jacobian to projective coordinates
func (p *{{ $TProjective }}) FromJacobian(Q *{{ $TJacobian }}) *{{ $TProjective }} {
	// memalloc
	var buf {{.CoordType}}
	buf.Square(&Q.Z)

	p.x.Mul(&Q.X, &Q.Z)
	p.y.Set(&Q.Y)
	p.z.Mul(&Q.Z, &buf)

	return p
}




{{/* note batch inversion for g2 elements with E2 that is curve specific is a bit more troublesome to implement */}}
{{if eq .PointName "g1"}}

// BatchJacobianToAffine{{ $TAffine }} converts points in Jacobian coordinates to Affine coordinates
// performing a single field inversion (Montgomery batch inversion trick)
// result must be allocated with len(result) == len(points)
func BatchJacobianToAffine{{ $TAffine }}(points []{{ $TJacobian }}, result []{{ $TAffine }}) {
	zeroes := make([]bool, len(points))
	accumulator := fp.One()

	// batch invert all points[].Z coordinates with Montgomery batch inversion trick
	// (stores points[].Z^-1 in result[i].X to avoid allocating a slice of fr.Elements)
	for i:=0; i < len(points); i++ {
		if points[i].Z.IsZero() {
			zeroes[i] = true
			continue
		}
		result[i].X = accumulator
		accumulator.Mul(&accumulator, &points[i].Z)
	}

	var accInverse fp.Element
	accInverse.Inverse(&accumulator)

	for i := len(points) - 1; i >= 0; i-- {
		if zeroes[i] {
			// do nothing, X and Y are zeroes in affine.
			continue
		}
		result[i].X.Mul(&result[i].X, &accInverse)
		accInverse.Mul(&accInverse, &points[i].Z)
	}

	// batch convert to affine.
	parallel.Execute( len(points), func(start, end int) {
		for i:=start; i < end; i++ {
			if zeroes[i] {
				// do nothing, X and Y are zeroes in affine.
				continue
			}
			var a, b fp.Element
			a = result[i].X
			b.Square(&a)
			result[i].X.Mul(&points[i].X, &b)
			result[i].Y.Mul(&points[i].Y, &b).
				Mul(&result[i].Y, &a)
		}
	})

}
{{end}}


// BatchScalarMultiplication{{ toUpper .PointName }} multiplies the same base (generator) by all scalars
// and return resulting points in affine coordinates
// uses a simple windowed-NAF like exponentiation algorithm
func BatchScalarMultiplication{{ toUpper .PointName }}(base *{{ $TAffine }}, scalars []fr.Element) []{{ $TAffine }} {

	// approximate cost in group ops is
	// cost = 2^{c-1} + n(scalar.nbBits+nbChunks)

	nbPoints := uint64(len(scalars))
	min := ^uint64(0)
	bestC := 0
	for c := 2; c < 18; c++  {
		cost := uint64(1 << (c-1))
		nbChunks := uint64(fr.Limbs * 64 / c)
		if (fr.Limbs*64) %c != 0 {
			nbChunks++
		}
		cost += nbPoints*((fr.Limbs*64) + nbChunks)
		if cost < min {
			min = cost
			bestC = c 
		}
	}
	c := uint64(bestC) // window size
	nbChunks := int(fr.Limbs * 64 / c)
	if (fr.Limbs*64) %c != 0 {
		nbChunks++
	}
	mask := uint64((1 << c) - 1)	// low c bits are 1
	msbWindow := uint64(1 << (c -1)) 

	// precompute all powers of base for our window
	// note here that if performance is critical, we can implement as in the msmX methods
	// this allocation to be on the stack
	baseTable := make([]{{ $TJacobian }}, (1<<(c-1)))
	baseTable[0].Set(&{{ toLower .PointName}}Infinity)
	baseTable[0].AddMixed(base)
	for i:=1;i<len(baseTable);i++ {
		baseTable[i] = baseTable[i-1]
		baseTable[i].AddMixed(base)
	}

	pScalars := partitionScalars(scalars, c)

	// compute offset and word selector / shift to select the right bits of our windows
	selectors := make([]selector, nbChunks)
	for chunk:=0; chunk < nbChunks; chunk++ {
		jc := uint64(uint64(chunk) * c)
		d := selector{}
		d.index = jc / 64
		d.shift = jc - (d.index * 64)
		d.mask = mask << d.shift
		d.multiWordSelect = (64%c) != 0  && d.shift > (64-c) && d.index < (fr.Limbs - 1 )
		if d.multiWordSelect {
			nbBitsHigh := d.shift - uint64(64-c)
			d.maskHigh = (1 << nbBitsHigh) - 1
			d.shiftHigh = (c - nbBitsHigh)
		}
		selectors[chunk] = d
	}

	{{if eq .PointName "g1"}}
		// convert our base exp table into affine to use AddMixed
		baseTableAff := make([]{{ $TAffine }}, (1<<(c-1)))
		BatchJacobianToAffine{{ $TAffine }}(baseTable, baseTableAff)
		toReturn := make([]{{ $TJacobian }}, len(scalars))
	{{else}}
		toReturn := make([]{{ $TAffine }}, len(scalars))
	{{end}}

	// for each digit, take value in the base table, double it c time, voila.
	parallel.Execute( len(pScalars), func(start, end int) {
		var p {{ $TJacobian }}
		for i:=start; i < end; i++ {
			p.Set(&{{ toLower .PointName}}Infinity)
			for chunk := nbChunks - 1; chunk >=0; chunk-- {
				s := selectors[chunk]
				if chunk != nbChunks -1 {
					for j:=uint64(0); j<c; j++ {
						p.DoubleAssign()
					}
				}

				bits := (pScalars[i][s.index] & s.mask) >> s.shift
				if s.multiWordSelect {
					bits += (pScalars[i][s.index+1] & s.maskHigh) << s.shiftHigh
				}

				if bits == 0 {
					continue
				}
				
				// if msbWindow bit is set, we need to substract
				if bits & msbWindow == 0 {
					// add 
					{{if eq .PointName "g1"}}
						p.AddMixed(&baseTableAff[bits-1])
					{{else}}
						p.AddAssign(&baseTable[bits-1])
					{{end}}
				} else {
					// sub
					{{if eq .PointName "g1"}}
						t := baseTableAff[bits & ^msbWindow]
						t.Neg(&t)
						p.AddMixed(&t)
					{{else}}
						t := baseTable[bits & ^msbWindow]
						t.Neg(&t)
						p.AddAssign(&t)
					{{end}}
				}
			}

			// set our result point 
			{{if eq .PointName "g1"}}
				toReturn[i] = p
			{{else}}
				toReturn[i].FromJacobian(&p)
			{{end}}
			
		}
	})

	{{if eq .PointName "g1"}}
		toReturnAff := make([]{{ $TAffine }}, len(scalars))
		BatchJacobianToAffine{{ $TAffine }}(toReturn, toReturnAff)
		return toReturnAff
	{{else}}
		return toReturn
	{{end}}
}



{{- $sizeOfFp := mul .Fp.NbWords 8}}


// SizeOf{{ $TAffine }}Compressed represents the size in bytes that a {{ $TAffine }} need in binary form, compressed
const SizeOf{{ $TAffine }}Compressed = {{ $sizeOfFp }} {{- if eq .CoordType "fptower.E2"}} * 2 {{- end}}

// SizeOf{{ $TAffine }}Uncompressed represents the size in bytes that a {{ $TAffine }} need in binary form, uncompressed
const SizeOf{{ $TAffine }}Uncompressed = SizeOf{{ $TAffine }}Compressed * 2



// Marshal converts p to a byte slice (without point compression)
func (p *{{ $TAffine }}) Marshal() ([]byte) {
	b := p.RawBytes()
	return b[:]
}

// Unmarshal is an allias to SetBytes()
func (p *{{ $TAffine }}) Unmarshal(buf []byte) error {
	_, err := p.SetBytes(buf)
	return err 
}




// Bytes returns binary representation of p
// will store X coordinate in regular form and a parity bit
{{- if ge .FpUnusedBits 3}}
// we follow the BLS381 style encoding as specified in ZCash and now IETF
// The most significant bit, when set, indicates that the point is in compressed form. Otherwise, the point is in uncompressed form.
// The second-most significant bit indicates that the point is at infinity. If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is the lexicographically largest of the two associated with the encoded x-coordinate.
{{- else}}
// as we have less than 3 bits available in our coordinate, we can't follow BLS381 style encoding (ZCash/IETF)
// we use the 2 most significant bits instead
// 00 -> uncompressed
// 10 -> compressed, use smallest lexicographically square root of Y^2
// 11 -> compressed, use largest lexicographically square root of Y^2
// 01 -> compressed infinity point
// the "uncompressed infinity point" will just have 00 (uncompressed) followed by zeroes (infinity = 0,0 in affine coordinates)
{{- end}}
func (p *{{ $TAffine }}) Bytes() (res [SizeOf{{ $TAffine }}Compressed]byte) {

	// check if p is infinity point
	if p.X.IsZero() && p.Y.IsZero() {
		res[0] = mCompressedInfinity
		return
	}

	// tmp is used to convert from montgomery representation to regular
	var tmp fp.Element

	msbMask := mCompressedSmallest
	// compressed, we need to know if Y is lexicographically bigger than -Y
	// if p.Y ">" -p.Y 
	if p.Y.LexicographicallyLargest() { 
		msbMask = mCompressedLargest
	}

	// we store X  and mask the most significant word with our metadata mask
	{{- if eq .CoordType "fptower.E2"}}
		// p.X.A0 | p.X.A1
		{{- $offset := $sizeOfFp}}
		{{- template "putFp" dict "all" . "OffSet" $offset "From" "p.X.A0"}}
		{{- template "putFp" dict "all" . "OffSet" 0 "From" "p.X.A1"}}
	{{- else}}
		{{- template "putFp" dict "all" . "OffSet" 0 "From" "p.X"}}
	{{- end}}

	res[0] |= msbMask

	return
}


// RawBytes returns binary representation of p (stores X and Y coordinate)
// see Bytes() for a compressed representation
func (p *{{ $TAffine }}) RawBytes() (res [SizeOf{{ $TAffine }}Uncompressed]byte) {

	// check if p is infinity point
	if p.X.IsZero() && p.Y.IsZero() {
		{{if ge .FpUnusedBits 3}}
			res[0] = mUncompressedInfinity
		{{else}}
			res[0] = mUncompressed 
		{{end}}
		return
	}

	// tmp is used to convert from montgomery representation to regular
	var tmp fp.Element

	// not compressed
	// we store the Y coordinate
	{{- if eq .CoordType "fptower.E2"}}
		// p.Y.A0 | p.Y.A1
		{{- $offset := mul $sizeOfFp 3}}
		{{- template "putFp" dict "all" . "OffSet" $offset "From" "p.Y.A0"}}

		{{- $offset := mul $sizeOfFp 2}}
		{{- template "putFp" dict "all" . "OffSet" $offset "From" "p.Y.A1"}}
	{{- else}}
		{{- template "putFp" dict "all" . "OffSet" $sizeOfFp "From" "p.Y"}}
	{{- end}}

	// we store X  and mask the most significant word with our metadata mask
	{{- if eq .CoordType "fptower.E2"}}
		// p.X.A0 | p.X.A1
		{{- $offset := $sizeOfFp}}
		{{- template "putFp" dict "all" . "OffSet" 0 "From" "p.X.A1"}}
		{{- template "putFp" dict "all" . "OffSet" $offset "From" "p.X.A0"}}
		
	{{- else}}
		{{- template "putFp" dict "all" . "OffSet" 0 "From" "p.X"}}
	{{- end}}

	res[0] |= mUncompressed

	return 
}

// SetBytes sets p from binary representation in buf and returns number of consumed bytes
// bytes in buf must match either RawBytes() or Bytes() output
// if buf is too short io.ErrShortBuffer is returned
// if buf contains compressed representation (output from Bytes()) and we're unable to compute
// the Y coordinate (i.e the square root doesn't exist) this function retunrs an error
// note that this doesn't check if the resulting point is on the curve or in the correct subgroup
func (p *{{ $TAffine }}) SetBytes(buf []byte) (int, error)  {
	if len(buf) < SizeOf{{ $TAffine }}Compressed {
		return 0, io.ErrShortBuffer
	}

	// most significant byte
	mData := buf[0] & mMask
	

	// check buffer size
	if (mData == mUncompressed) {{- if ge .FpUnusedBits 3}} || (mData == mUncompressedInfinity) {{- end}}  {
		if len(buf) < SizeOf{{ $TAffine }}Uncompressed {
			return 0, io.ErrShortBuffer
		}
	} 

	// if infinity is encoded in the metadata, we don't need to read the buffer
	if (mData == mCompressedInfinity) {
		p.X.SetZero()
		p.Y.SetZero()
		return SizeOf{{ $TAffine }}Compressed, nil
	}

	{{- if ge .FpUnusedBits 3}} 
	if (mData == mUncompressedInfinity) {
		p.X.SetZero()
		p.Y.SetZero()
		return SizeOf{{ $TAffine }}Uncompressed, nil
	}
	{{- end}} 

	// TODO that's not elegant as it modifies buf; buf is now consumable only in 1 go routine
	buf[0] &= ^mMask 

	// read X coordinate
	{{- if eq .CoordType "fptower.E2"}}
		// p.X.A1 | p.X.A0
		{{- $offset := $sizeOfFp}}
		{{- template "readFp" dict "all" . "OffSet" $offset "To" "p.X.A0"}}
		{{- template "readFp" dict "all" . "OffSet" 0 "To" "p.X.A1"}}
	{{- else}}
		{{- template "readFp" dict "all" . "OffSet" 0 "To" "p.X"}}
	{{- end}}

	// restore buf
	buf[0] |= mData

	// uncompressed point
	if mData == mUncompressed {
		// read Y coordinate
		{{- if eq .CoordType "fptower.E2"}}
			// p.Y.A1 | p.Y.A0
			{{- $offset := mul $sizeOfFp 2}}
			{{- template "readFp" dict "all" . "OffSet" $offset "To" "p.Y.A1"}}

			{{- $offset := mul $sizeOfFp 3}}
			{{- template "readFp" dict "all" . "OffSet" $offset "To" "p.Y.A0"}}
			
		{{- else}}
			{{- template "readFp" dict "all" . "OffSet" $sizeOfFp "To" "p.Y"}}
		{{- end}}

		return SizeOf{{ $TAffine }}Uncompressed, nil
	}

	// we have a compressed coordinate, we need to solve the curve equation to compute Y
	var YSquared, Y {{.CoordType}}

	YSquared.Square(&p.X).Mul(&YSquared, &p.X)
	YSquared.Add(&YSquared, &{{- if eq .PointName "g2"}}bTwistCurveCoeff{{- else}}bCurveCoeff{{- end}})

	{{- if eq .CoordType "fptower.E2"}}
		if YSquared.Legendre() == -1 {
			return 0, errors.New("invalid compressed coordinate: square root doesn't exist")
		}
		Y.Sqrt(&YSquared)
	{{- else}}
		if Y.Sqrt(&YSquared) == nil {
			return 0, errors.New("invalid compressed coordinate: square root doesn't exist")
		}
	{{- end}}

	
	if Y.LexicographicallyLargest()  { 
		// Y ">" -Y
		if mData == mCompressedSmallest {
			Y.Neg(&Y)
		}
	} else {
		// Y "<=" -Y
		if mData == mCompressedLargest {
			Y.Neg(&Y)
		}
	}

	p.Y = Y

	return SizeOf{{ $TAffine }}Compressed, nil 
}



// unsafeComputeY called by Decoder when processing slices of compressed point in parallel (step 2)
// it computes the Y coordinate from the already set X coordinate and is compute intensive
func (p *{{ $TAffine }}) unsafeComputeY() error  {
	// stored in unsafeSetCompressedBytes
	{{ if eq .CoordType "fptower.E2"}}
	mData := byte(p.Y.A0[0])
	{{ else}}
	mData := byte(p.Y[0])
	{{ end}}


	// we have a compressed coordinate, we need to solve the curve equation to compute Y
	var YSquared, Y {{.CoordType}}

	YSquared.Square(&p.X).Mul(&YSquared, &p.X)
	YSquared.Add(&YSquared, &{{- if eq .PointName "g2"}}bTwistCurveCoeff{{- else}}bCurveCoeff{{- end}})

	{{- if eq .CoordType "fptower.E2"}}
		if YSquared.Legendre() == -1 {
			return errors.New("invalid compressed coordinate: square root doesn't exist")
		}
		Y.Sqrt(&YSquared)
	{{- else}}
		if Y.Sqrt(&YSquared) == nil {
			return errors.New("invalid compressed coordinate: square root doesn't exist")
		}
	{{- end}}

	
	if Y.LexicographicallyLargest()  { 
		// Y ">" -Y
		if mData == mCompressedSmallest {
			Y.Neg(&Y)
		}
	} else {
		// Y "<=" -Y
		if mData == mCompressedLargest {
			Y.Neg(&Y)
		}
	}

	p.Y = Y

	return nil
}

// unsafeSetCompressedBytes is called by Decoder when processing slices of compressed point in parallel (step 1)
// assumes buf[:8] mask is set to compressed
// returns true if point is infinity and need no further processing
// it sets X coordinate and uses Y for scratch space to store decompression metadata
func (p *{{ $TAffine }}) unsafeSetCompressedBytes(buf []byte) (isInfinity bool)  {

	// read the most significant byte
	mData := buf[0] & mMask
	
	if (mData == mCompressedInfinity) {
		p.X.SetZero()
		p.Y.SetZero()
		isInfinity = true
		return
	}

	// read X

	// TODO that's not elegant as it modifies buf; buf is now consumable only in 1 go routine
	buf[0] &= ^mMask 

	// read X coordinate
	{{- if eq .CoordType "fptower.E2"}}
		// p.X.A1 | p.X.A0
		{{- $offset := $sizeOfFp}}
		{{- template "readFp" dict "all" . "OffSet" 0 "To" "p.X.A1"}}
		{{- template "readFp" dict "all" . "OffSet"  $offset "To" "p.X.A0"}}
	{{- else}}
		{{- template "readFp" dict "all" . "OffSet" 0 "To" "p.X"}}
	{{- end}}

	// restore buf
	buf[0] |= mData

	{{ if eq .CoordType "fptower.E2"}}
	// store mData in p.Y.A0[0]
	p.Y.A0[0] = uint64(mData)
	{{ else}}
	// store mData in p.Y[0]
	p.Y[0] = uint64(mData)
	{{ end}}

	// recomputing Y will be done asynchronously
	return
}



{{define "putFp"}}
	tmp = {{$.From}}
	tmp.FromMont() 
	{{- range $i := reverse .all.Fp.NbWordsIndexesFull}}
			{{- $j := mul $i 8}}
			{{- $j := add $j $.OffSet}}
			{{- $k := sub $.all.Fp.NbWords 1}}
			{{- $k := sub $k $i}}
			{{- $jj := add $j 8}}
			binary.BigEndian.PutUint64(res[{{$j}}:{{$jj}}], tmp[{{$k}}])
	{{- end}}
{{end}}

{{define "readFp"}}
	{{$.To}}.SetBytes(buf[{{$.OffSet}}:{{$.OffSet}} + fp.Bytes])
{{end}}

`
