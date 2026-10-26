package auth

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

// sm3.go — SM3 密码杂凑算法本地实现，按 GB/T 32905-2016 直译。
//
// 选型决策（PR 正文有完整取舍）：本项目只消费一个 hash.Hash 构造函数（被
// password.go 的算法注册表引用），算法自 2016 年冻结不存在演进问题，国标自带
// 官方测试向量（sm3_test.go 硬编码锁死，实现错误必然被单测抓出），因此不引入
// 第三方国密库，保持全 PR 零新增依赖。开发期曾与 gmsm v0.45.1 输出对拍验证。
// 纯 Go 版未做 plan9 汇编优化（amd64/arm64），原因与将来再议的后门见 PR 正文：
// 登录延迟预算靠迭代数定档调节，注册表换实现只需一行。

const (
	sm3Size      = 32 // 摘要长度（字节）
	sm3BlockSize = 64 // 分组长度（字节）
)

// sm3IV 初始值 IV（标准 §5.3.2）。
var sm3IV = [8]uint32{
	0x7380166f, 0x4914b2b9, 0x172442d7, 0xda8a0600,
	0xa96f30bc, 0x163138aa, 0xe38dee4d, 0xb0fb0e4e,
}

// sm3Digest 增量写入中的哈希状态。
type sm3Digest struct {
	state  [8]uint32
	x      [sm3BlockSize]byte // 未满一组的输入缓冲
	nx     int                // 缓冲内字节数
	length uint64             // 已写入总字节数
}

// sm3New 返回计算 SM3 的 hash.Hash（可 Reset 后复用）。
func sm3New() hash.Hash {
	d := new(sm3Digest)
	d.Reset()
	return d
}

func (d *sm3Digest) Reset() {
	d.state = sm3IV
	d.nx = 0
	d.length = 0
}

func (d *sm3Digest) Size() int      { return sm3Size }
func (d *sm3Digest) BlockSize() int { return sm3BlockSize }

// Write 增量写入。hash.Hash 契约：返回消耗的字节数，本实现不产生写入错误。
func (d *sm3Digest) Write(p []byte) (int, error) {
	total := len(p)
	d.length += uint64(total)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		p = p[n:]
		if d.nx == sm3BlockSize {
			d.compress(d.x[:])
			d.nx = 0
		}
	}
	for len(p) >= sm3BlockSize {
		d.compress(p)
		p = p[sm3BlockSize:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return total, nil
}

// Sum 追加当前摘要到 in 并返回，不破坏哈希状态（后续可继续 Write）。
// 填充在状态副本上做：0x80 + 零 + 64 位大端比特长（标准 §5.2）。
func (d *sm3Digest) Sum(in []byte) []byte {
	t := *d
	r := int(t.length % sm3BlockSize)
	plen := 9 + (55-r+64)%64 // 1 字节 0x80 + k 字节零 + 8 字节长度（按实际长度开，喂完即空）
	pad := make([]byte, plen)
	pad[0] = 0x80
	binary.BigEndian.PutUint64(pad[plen-8:plen], t.length<<3)
	if t.nx > 0 {
		n := copy(t.x[t.nx:], pad[:])
		t.nx += n
		pad = pad[n:]
		if t.nx == sm3BlockSize {
			t.compress(t.x[:])
			t.nx = 0
		}
	}
	for len(pad) >= sm3BlockSize {
		t.compress(pad)
		pad = pad[sm3BlockSize:]
	}
	if len(pad) > 0 {
		var last [sm3BlockSize]byte
		copy(last[:], pad)
		t.compress(last[:])
	}
	for i := 0; i < 8; i++ {
		in = binary.BigEndian.AppendUint32(in, t.state[i])
	}
	return in
}

// sm3P1 消息扩展用置换（标准 §4.2）。
func sm3P1(x uint32) uint32 { return x ^ bits.RotateLeft32(x, 15) ^ bits.RotateLeft32(x, 23) }

// sm3P0 压缩函数用置换（标准 §4.1）。
func sm3P0(x uint32) uint32 { return x ^ bits.RotateLeft32(x, 9) ^ bits.RotateLeft32(x, 17) }

// compress 压缩一个 64 字节分组（只读 block 前 64 字节），把轮函数输出异或进状态。
// SM3 链接值为异或累积（区别于 SHA-256 的加法）。
func (d *sm3Digest) compress(block []byte) {
	var w [68]uint32
	var w1 [64]uint32
	for i := 0; i < 16; i++ {
		w[i] = binary.BigEndian.Uint32(block[4*i:])
	}
	for i := 16; i < 68; i++ {
		w[i] = sm3P1(w[i-16]^w[i-9]^bits.RotateLeft32(w[i-3], 15)) ^
			bits.RotateLeft32(w[i-13], 7) ^ w[i-6]
	}
	for i := 0; i < 64; i++ {
		w1[i] = w[i] ^ w[i+4]
	}

	a, b, c, dd, e, f, g, h := d.state[0], d.state[1], d.state[2], d.state[3],
		d.state[4], d.state[5], d.state[6], d.state[7]

	for j := 0; j < 64; j++ {
		var t, ff, gg uint32
		if j < 16 {
			t = 0x79cc4519
			ff = a ^ b ^ c
			gg = e ^ f ^ g
		} else {
			t = 0x7a879d8a
			ff = (a & b) | (a & c) | (b & c)
			gg = (e & f) | (^e & g)
		}
		a12 := bits.RotateLeft32(a, 12)
		// 标准轮函数：SS1 = ((A<<<12) + E + (Tj<<<(j mod 32))) <<< 7 —— E 是轮开始
		// 时的寄存器值，参与 SS1（与 tjfoc/gmsm 两家实现及国标 KAT 一致）。
		ss1 := bits.RotateLeft32(a12+e+bits.RotateLeft32(t, int(j%32)), 7)
		ss2 := ss1 ^ a12
		tt1 := ff + dd + ss2 + w1[j]
		tt2 := gg + h + ss1 + w[j]
		dd = c
		c = bits.RotateLeft32(b, 9)
		b = a
		a = tt1
		h = g
		g = bits.RotateLeft32(f, 19)
		f = e
		e = sm3P0(tt2)
	}

	d.state[0] ^= a
	d.state[1] ^= b
	d.state[2] ^= c
	d.state[3] ^= dd
	d.state[4] ^= e
	d.state[5] ^= f
	d.state[6] ^= g
	d.state[7] ^= h
}
