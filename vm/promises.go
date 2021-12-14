package vm

import (
	"math/big"
	"github.com/holiman/uint256"
	"sync"
)

type IntPromise struct {
	wg    sync.WaitGroup
	value *uint256.Int
}

func (p *IntPromise) resolve() *uint256.Int {
	if p == nil {
		return new(uint256.Int)
	}
	p.wg.Wait()
	return p.value
}

func (p *IntPromise) Then(fn func(*uint256.Int) *uint256.Int) *IntPromise {
	next := &IntPromise{}
	next.wg.Add(1)
	go func() {
		next.value = fn(p.resolve())
		next.wg.Done()
	}()
	return next
}

func (p *IntPromise) ThenP(fn func(*uint256.Int) *IntPromise) *IntPromise {
	next := new(IntPromise)
	next.wg.Add(1)
	go func() {
		next.value = fn(p.resolve()).resolve()
		next.wg.Done()
	}()
	return next
}

type Int struct {
	p *IntPromise
}

func NewInt(x uint64) *Int {
	return &Int{p: &IntPromise{value: uint256.NewInt(x)}}
}

func IntFromU256(x uint256.Int) *Int {
	return &Int{p: &IntPromise{value: &x}}
}

func IntFromBig(x *big.Int) *Int {
	v, _ := uint256.FromBig(x)
	return &Int{p: &IntPromise{value: v}}
}

func (z *Int) UpdateBytes(fn func() []byte) {
	z.p = &IntPromise{}
	z.p.wg.Add(1)
	go func() {
		z.p.value = new(uint256.Int).SetBytes(fn())
		z.p.wg.Done()
	}()
}


func (z *Int) SetBytes(buf []byte) *Int {
	z.p = &IntPromise{value: new(uint256.Int).SetBytes(buf)}
	return z
}
func (z *Int) SetFromBig(x *big.Int) *Int {
	v := new(uint256.Int)
	v.SetFromBig(x)
	z.p = &IntPromise{value: v}
	return z
}
func (z *Int) Bytes32() [32]byte {
	return z.p.resolve().Bytes32()
}
func (z *Int) Bytes20() [20]byte {
	return z.p.resolve().Bytes20()
}
func (z *Int) Bytes() []byte {
	return z.p.resolve().Bytes()
}
func (z *Int) WriteToSlice(dest []byte) {
	z.p.resolve().WriteToSlice(dest)
}
func (z *Int) WriteToArray32(dest *[32]byte) {
	z.p.resolve().WriteToArray32(dest)
}
func (z *Int) WriteToArray20(dest *[20]byte) {
	z.p.resolve().WriteToArray20(dest)
}
func (z *Int) Uint64() uint64 {
	return z.p.resolve().Uint64()
}
func (z *Int) Uint64WithOverflow() (uint64, bool) {
	return z.p.resolve().Uint64WithOverflow()
}
func (z *Int) Uint256() *uint256.Int {
	return z.p.resolve()
}
func (z *Int) ToBig() *big.Int {
	return z.p.resolve().ToBig()
}
func (z *Int) Clone() *Int {
	return &Int{p: z.p}
}
func (z *Int) Add(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).Add(xv, yv)
		})
	})
	return z
}
func (z *Int) AddOverflow(x, y *Int) (*Int, bool) {
	zv, v := new(uint256.Int).AddOverflow(x.p.resolve(), y.p.resolve())
	z.p = &IntPromise{value: zv}
	return z, v
}
func (z *Int) AddMod(x, y, m *Int) *Int {
	yp, mp := y.p, m.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.ThenP(func(yv *uint256.Int) *IntPromise {
			return mp.Then(func(mv *uint256.Int) *uint256.Int {
				if mv.IsZero() {
					return new(uint256.Int)
				}
				return new(uint256.Int).AddMod(xv, yv, mv)
			})
		})
	})
	return z
}
func (z *Int) AddUint64(x *Int, y uint64) *Int {
	z.p = x.p.Then(func( xv *uint256.Int) *uint256.Int {
		return new(uint256.Int).AddUint64(xv, y)
	})
	return z
}
func (z *Int) PaddedBytes(n int) []byte {
	return z.p.resolve().PaddedBytes(n)
}
func (z *Int) SubUint64(x *Int, y uint64) *Int {
	z.p = x.p.Then(func(xv *uint256.Int) *uint256.Int {
		return new(uint256.Int).SubUint64(xv, y)
	})
	return z
}
func (z *Int) SubOverflow(x, y *Int) (*Int, bool) {
	zv, v := new(uint256.Int).SubOverflow(x.p.resolve(), y.p.resolve())
	z.p = &IntPromise{value: zv}
	return z, v
}
func (z *Int) Sub(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).Sub(xv, yv)
		})
	})
	return z
}
func (z *Int) Mul(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).Mul(xv, yv)
		})
	})
	return z
}
func (z *Int) MulOverflow(x, y *Int) (*Int, bool) {
	zv, v := new(uint256.Int).MulOverflow(x.p.resolve(), y.p.resolve())
	z.p = &IntPromise{value: zv}
	return z, v
}
func (z *Int) squared() {
	z.p = z.p.Then(func(zv *uint256.Int) *uint256.Int {
		return new(uint256.Int).Mul(zv, zv)
	})
}
func (z *Int) Div(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).Div(xv, yv)
		})
	})
	return z
}
func (z *Int) Mod(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).Mod(xv, yv)
		})
	})
	return z
}
func (z *Int) SMod(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).SMod(xv, yv)
		})
	})
	return z
}
func (z *Int) MulMod(x, y, m *Int) *Int {
	yp, mp := y.p, m.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.ThenP(func(yv *uint256.Int) *IntPromise {
			return mp.Then(func(mv *uint256.Int) *uint256.Int {
				return new(uint256.Int).MulMod(xv, yv, mv)
			})
		})
	})
	return z
}
func (z *Int) Abs(x *Int) *Int {
	z.p = x.p.Then(func(xv *uint256.Int) *uint256.Int {
		return new(uint256.Int).Abs(xv)
	})
	return z
}
func (z *Int) Neg(x *Int) *Int {
	z.p = x.p.Then(func(xv *uint256.Int) *uint256.Int {
		return new(uint256.Int).Neg(xv)
	})
	return z
}
func (z *Int) SDiv(n, d *Int) *Int {
	dp := d.p
	z.p = n.p.ThenP(func(nv *uint256.Int) *IntPromise {
		return dp.Then(func(dv *uint256.Int) *uint256.Int {
			return new(uint256.Int).SDiv(nv, dv)
		})
	})
	return z
}
func (z *Int) Sign() int {
	return z.p.resolve().Sign()
}
func (z *Int) BitLen() int {
	return z.p.resolve().BitLen()
}
func (z *Int) ByteLen() int {
	return z.p.resolve().ByteLen()
}
func (z *Int) Not(x *Int) *Int {
	z.p = x.p.Then(func(xv *uint256.Int) *uint256.Int {
		return new(uint256.Int).Not(xv)
	})
	return z
}
func (z *Int) Gt(x *Int) bool {
	return z.p.resolve().Gt(x.p.resolve())
}
func (z *Int) IsGt(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			if xv.Gt(yv) {
				return new(uint256.Int).SetOne()
			}
			return new(uint256.Int).Clear()
		})
	})
	return z
}
func (z *Int) Slt(x *Int) bool {
	return z.p.resolve().Slt(x.p.resolve())
}
func (z *Int) IsSlt(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			if xv.Slt(yv) {
				return new(uint256.Int).SetOne()
			}
			return new(uint256.Int).Clear()
		})
	})
	return z
}
func (z *Int) Sgt(x *Int) bool {
	return z.p.resolve().Sgt(x.p.resolve())
}
func (z *Int) IsSgt(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			if xv.Sgt(yv) {
				return new(uint256.Int).SetOne()
			}
			return new(uint256.Int).Clear()
		})
	})
	return z
}
func (z *Int) Lt(x *Int) bool {
	return z.p.resolve().Lt(x.p.resolve())
}
func (z *Int) IsLt(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			if xv.Lt(yv) {
				return new(uint256.Int).SetOne()
			}
			return new(uint256.Int).Clear()
		})
	})
	return z
}
func (z *Int) SetUint64(x uint64) *Int {
	z.p = &IntPromise{value: new(uint256.Int).SetUint64(x)}
	return z
}
func (z *Int) Eq(x *Int) bool {
	return z.p.resolve().Eq(x.p.resolve())
}
func (z *Int) IsEq(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			if xv.Eq(yv) {
				return new(uint256.Int).SetOne()
			}
			return new(uint256.Int).Clear()
		})
	})
	return z
}
func (z *Int) Cmp(x *Int) (r int) {
	return z.p.resolve().Cmp(x.p.resolve())
}
func (z *Int) LtUint64(n uint64) bool {
	return z.p.resolve().LtUint64(n)
}
func (z *Int) GtUint64(n uint64) bool {
	return z.p.resolve().GtUint64(n)
}
func (z *Int) IsUint64() bool {
	return z.p.resolve().IsUint64()
}
func (z *Int) IsZero() bool {
	return z.p.resolve().IsZero()
}
func (z *Int) CheckIsZero(x *Int) *Int {
	z.p = x.p.Then(func(xv *uint256.Int) *uint256.Int {
		if xv.IsZero() {
			return new(uint256.Int).SetOne()
		}
		return new(uint256.Int).Clear()
	})
	return z
}
func (z *Int) Clear() *Int {
	z.p = &IntPromise{value: new(uint256.Int).Clear()}
	return z
}
func (z *Int) SetAllOne() *Int {
	z.p = &IntPromise{value: new(uint256.Int).SetAllOne()}
	return z
}
func (z *Int) SetOne() *Int {
	z.p = &IntPromise{value: new(uint256.Int).SetOne()}
	return z
}
func (z *Int) Lsh(x *Int, n uint) *Int {
	z.p = x.p.Then(func( xv *uint256.Int) *uint256.Int {
		return new(uint256.Int).Lsh(xv, n)
	})
	return z
}
func (z *Int) Rsh(x *Int, n uint) *Int {
	z.p = x.p.Then(func( xv *uint256.Int) *uint256.Int {
		return new(uint256.Int).Rsh(xv, n)
	})
	return z
}
func (z *Int) SRsh(x *Int, n uint) *Int {
	z.p = x.p.Then(func( xv *uint256.Int) *uint256.Int {
		return new(uint256.Int).SRsh(xv, n)
	})
	return z
}
func (z *Int) Set(x *Int) *Int {
	z.p = x.p
	return z
}
func (z *Int) Or(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).Or(xv, yv)
		})
	})
	return z
}
func (z *Int) And(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).And(xv, yv)
		})
	})
	return z
}
func (z *Int) Xor(x, y *Int) *Int {
	yp := y.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return yp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).Xor(xv, yv)
		})
	})
	return z
}
func (z *Int) Byte(n *Int) *Int {
	np := n.p
	z.p = z.p.ThenP(func(zv *uint256.Int) *IntPromise {
		return np.Then(func(nv *uint256.Int) *uint256.Int {
			return zv.Byte(nv)
		})
	})
	return z
}
func (z *Int) Exp(base, exponent *Int) *Int {
	ep := exponent.p
	z.p = base.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return ep.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).Exp(xv, yv)
		})
	})
	return z
}
func (z *Int) ExtendSign(x, byteNum *Int) *Int {
	bp := byteNum.p
	z.p = x.p.ThenP(func(xv *uint256.Int) *IntPromise {
		return bp.Then(func(yv *uint256.Int) *uint256.Int {
			return new(uint256.Int).ExtendSign(xv, yv)
		})
	})
	return z
}
