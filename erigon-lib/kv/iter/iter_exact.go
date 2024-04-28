package iter

import (
	"bytes"
)

// often used shortcuts
type (
	U64 Uno[uint64]
	KV  Duo[[]byte, []byte]          // key,  value
	KVS Trio[[]byte, []byte, uint64] // key, value, step
)

var (
	EmptyU64 = &Empty[uint64]{}
	EmptyKV  = &Empty2[[]byte, []byte]{}
	EmptyKVS = &Empty3[[]byte, []byte, uint64]{}
)

func FilterU64(it U64, filter func(k uint64) bool) *Filtered[uint64] {
	return Filter[uint64](it, filter)
}
func FilterKV(it KV, filter func(k, v []byte) bool) *Filtered2[[]byte, []byte] {
	return Filter2[[]byte, []byte](it, filter)
}

func ToU64Arr(s U64) ([]uint64, error)           { return ToArr[uint64](s) }
func ToKVArray(s KV) ([][]byte, [][]byte, error) { return ToArr2[[]byte, []byte](s) }

func ToArrU64Must(s U64) []uint64 {
	arr, err := ToArr[uint64](s)
	if err != nil {
		panic(err)
	}
	return arr
}
func ToArrKVMust(s KV) ([][]byte, [][]byte) {
	keys, values, err := ToArr2[[]byte, []byte](s)
	if err != nil {
		panic(err)
	}
	return keys, values
}

func CountU64(s U64) (int, error) { return Count[uint64](s) }
func CountKV(s KV) (int, error)   { return Count2[[]byte, []byte](s) }

func TransformKV(it KV, transform func(k, v []byte) ([]byte, []byte, error)) *Transformed2[[]byte, []byte] {
	return Transform2[[]byte, []byte](it, transform)
}

// internal types
type (
	NextPage1[T any]    func(pageToken string) (arr []T, nextPageToken string, err error)
	NextPage2[K, V any] func(pageToken string) (keys []K, values []V, nextPageToken string, err error)
)

func PaginateKV(f NextPage2[[]byte, []byte]) *Paginated2[[]byte, []byte] {
	return Paginate2[[]byte, []byte](f)
}
func PaginateU64(f NextPage1[uint64]) *Paginated[uint64] {
	return Paginate[uint64](f)
}

type TransformKV2U64Iter[K, V []byte] struct {
	it        KV
	transform func(K, V) (uint64, error)
}

func TransformKV2U64[K, V []byte](it KV, transform func(K, V) (uint64, error)) *TransformKV2U64Iter[K, V] {
	return &TransformKV2U64Iter[K, V]{it: it, transform: transform}
}
func (m *TransformKV2U64Iter[K, V]) HasNext() bool { return m.it.HasNext() }
func (m *TransformKV2U64Iter[K, V]) Next() (uint64, error) {
	k, v, err := m.it.Next()
	if err != nil {
		return 0, err
	}
	return m.transform(k, v)
}
func (m *TransformKV2U64Iter[K, v]) Close() {
	if x, ok := m.it.(Closer); ok {
		x.Close()
	}
}

// UnionKVIter - merge 2 kv.Pairs streams to 1 in lexicographically order
// 1-st stream has higher priority - when 2 streams return same key
type UnionKVIter struct {
	x, y               KV
	xHasNext, yHasNext bool
	xNextK, xNextV     []byte
	yNextK, yNextV     []byte
	limit              int
	err                error
}

func UnionKV(x, y KV, limit int) KV {
	if x == nil && y == nil {
		return EmptyKV
	}
	if x == nil {
		return y
	}
	if y == nil {
		return x
	}
	m := &UnionKVIter{x: x, y: y, limit: limit}
	m.advanceX()
	m.advanceY()
	return m
}
func (m *UnionKVIter) HasNext() bool {
	return m.err != nil || (m.limit != 0 && m.xHasNext) || (m.limit != 0 && m.yHasNext)
}
func (m *UnionKVIter) advanceX() {
	if m.err != nil {
		return
	}
	m.xHasNext = m.x.HasNext()
	if m.xHasNext {
		m.xNextK, m.xNextV, m.err = m.x.Next()
	}
}
func (m *UnionKVIter) advanceY() {
	if m.err != nil {
		return
	}
	m.yHasNext = m.y.HasNext()
	if m.yHasNext {
		m.yNextK, m.yNextV, m.err = m.y.Next()
	}
}
func (m *UnionKVIter) Next() ([]byte, []byte, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	m.limit--
	if m.xHasNext && m.yHasNext {
		cmp := bytes.Compare(m.xNextK, m.yNextK)
		if cmp < 0 {
			k, v, err := m.xNextK, m.xNextV, m.err
			m.advanceX()
			return k, v, err
		} else if cmp == 0 {
			k, v, err := m.xNextK, m.xNextV, m.err
			m.advanceX()
			m.advanceY()
			return k, v, err
		}
		k, v, err := m.yNextK, m.yNextV, m.err
		m.advanceY()
		return k, v, err
	}
	if m.xHasNext {
		k, v, err := m.xNextK, m.xNextV, m.err
		m.advanceX()
		return k, v, err
	}
	k, v, err := m.yNextK, m.yNextV, m.err
	m.advanceY()
	return k, v, err
}

// func (m *UnionKVIter) ToArray() (keys, values [][]byte, err error) { return ToKVArray(m) }
func (m *UnionKVIter) Close() {
	if x, ok := m.x.(Closer); ok {
		x.Close()
	}
	if y, ok := m.y.(Closer); ok {
		y.Close()
	}
}

type WrapKVSIter struct {
	y              KV
	yHasNext       bool
	yNextK, yNextV []byte
	err            error
}

func WrapKVS(y KV) KVS {
	if y == nil {
		return EmptyKVS
	}
	m := &WrapKVSIter{y: y}
	m.advance()
	return m
}

func (m *WrapKVSIter) HasNext() bool {
	return m.err != nil || m.yHasNext
}
func (m *WrapKVSIter) advance() {
	if m.err != nil {
		return
	}
	m.yHasNext = m.y.HasNext()
	if m.yHasNext {
		m.yNextK, m.yNextV, m.err = m.y.Next()
	}
}
func (m *WrapKVSIter) Next() ([]byte, []byte, uint64, error) {
	if m.err != nil {
		return nil, nil, 0, m.err
	}
	k, v, err := m.yNextK, m.yNextV, m.err
	m.advance()
	return k, v, 0, err
}

// func (m *WrapKVSIter) ToArray() (keys, values [][]byte, err error) { return ToKVArray(m) }
func (m *WrapKVSIter) Close() {
	if y, ok := m.y.(Closer); ok {
		y.Close()
	}
}

type WrapKVIter struct {
	x              KVS
	xHasNext       bool
	xNextK, xNextV []byte
	err            error
}

func WrapKV(x KVS) KV {
	if x == nil {
		return EmptyKV
	}
	m := &WrapKVIter{x: x}
	m.advance()
	return m
}

func (m *WrapKVIter) HasNext() bool {
	return m.err != nil || m.xHasNext
}
func (m *WrapKVIter) advance() {
	if m.err != nil {
		return
	}
	m.xHasNext = m.x.HasNext()
	if m.xHasNext {
		m.xNextK, m.xNextV, _, m.err = m.x.Next()
	}
}
func (m *WrapKVIter) Next() ([]byte, []byte, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	k, v, err := m.xNextK, m.xNextV, m.err
	m.advance()
	return k, v, err
}

// func (m *WrapKVIter) ToArray() (keys, values [][]byte, err error) { return ToKVArray(m) }
func (m *WrapKVIter) Close() {
	if x, ok := m.x.(Closer); ok {
		x.Close()
	}
}

// MergedKV - merge 2 kv.Pairs streams (without replacements, or "shadowing",
// meaning that all input pairs will appear in the output stream - this is
// difference to UnionKVIter), to 1 in lexicographically order
// 1-st stream has higher priority - when 2 streams return same key
type MergedKV struct {
	x                  KVS
	y                  KV
	xHasNext, yHasNext bool
	xNextK, xNextV     []byte
	yNextK, yNextV     []byte
	xStep              uint64
	limit              int
	err                error
}

func MergeKVS(x KVS, y KV, limit int) KVS {
	if x == nil && y == nil {
		return EmptyKVS
	}
	if x == nil {
		return WrapKVS(y)
	}
	if y == nil {
		return x
	}
	m := &MergedKV{x: x, y: y, limit: limit}
	m.advanceX()
	m.advanceY()
	return m
}
func (m *MergedKV) HasNext() bool {
	return m.err != nil || (m.limit != 0 && m.xHasNext) || (m.limit != 0 && m.yHasNext)
}
func (m *MergedKV) advanceX() {
	if m.err != nil {
		return
	}
	m.xHasNext = m.x.HasNext()
	if m.xHasNext {
		m.xNextK, m.xNextV, m.xStep, m.err = m.x.Next()
	}
}
func (m *MergedKV) advanceY() {
	if m.err != nil {
		return
	}
	m.yHasNext = m.y.HasNext()
	if m.yHasNext {
		m.yNextK, m.yNextV, m.err = m.y.Next()
	}
}
func (m *MergedKV) Next() ([]byte, []byte, uint64, error) {
	if m.err != nil {
		return nil, nil, 0, m.err
	}
	m.limit--
	if m.xHasNext && m.yHasNext {
		cmp := bytes.Compare(m.xNextK, m.yNextK)
		if cmp <= 0 {
			k, v, step, err := m.xNextK, m.xNextV, m.xStep, m.err
			m.advanceX()
			return k, v, step, err
		}
		k, v, err := m.yNextK, m.yNextV, m.err
		m.advanceY()
		return k, v, 0, err
	}
	if m.xHasNext {
		k, v, step, err := m.xNextK, m.xNextV, m.xStep, m.err
		m.advanceX()
		return k, v, step, err
	}
	k, v, err := m.yNextK, m.yNextV, m.err
	m.advanceY()
	return k, v, 0, err
}

// func (m *MergedKV) ToArray() (keys, values [][]byte, err error) { return ToKVArray(m) }
func (m *MergedKV) Close() {
	if x, ok := m.x.(Closer); ok {
		x.Close()
	}
	if y, ok := m.y.(Closer); ok {
		y.Close()
	}
}

type Closer interface {
	Close()
}
