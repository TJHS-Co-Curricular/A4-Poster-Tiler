// Package winres 产生 exe 的 Windows 资源（F12 打包）：图标、版本信息（exe「属性 → 详细信息」）、manifest。
//
// 输出是 COFF 目标文件 rsrc_windows_amd64.syso，放在 cmd/a4-poster-tiler/ 旁边，go build 会自动带进 exe。
// 只用标准库，不需要 mingw 的 windres（Windows 上的 build.bat 也能跑）。
//
// 也负责画默认图标（MakeICO）和读 .ico 文件（ReadICO）。
package winres

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Info 是写进 exe「属性 → 详细信息」的内容。
type Info struct {
	Version     string // 1.0.1
	Company     string
	Description string
	Product     string
	FileName    string // A4-Poster-Tiler.exe
}

// IconImage 是 .ico 里的一个尺寸。Data 是 PNG 或 BMP（DIB，不含 BITMAPFILEHEADER）。
type IconImage struct {
	W, H     int // 256 写成 256（文件里存 0）
	BitCount uint16
	Data     []byte
}

const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
	rtManifest  = 24
	langZhCN    = 0x0804
)

// ---------- 图标 ----------

// IconSizes 是默认图标包含的尺寸。
var IconSizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

// DrawIcon 画默认图标：圆角青绿色方块 + 2 × 2 张白色「纸」（和网页 favicon.svg 同一个图案）。
func DrawIcon(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4 // 每个像素取 4 × 4 个点，边缘才平滑
	bg := color.NRGBA{0x0f, 0x8a, 0x7e, 0xff}
	k := float64(size) / 32 // 设计稿是 32 × 32
	type rr struct{ x, y, w, h, r float64 }
	outer := rr{0, 0, 32, 32, 6}
	tiles := []rr{{5, 6, 10, 9, 1}, {17, 6, 10, 9, 1}, {5, 17, 10, 9, 1}, {17, 17, 10, 9, 1}}
	if size <= 20 { // 太小时把白色纸放大一点，比较看得出是四张
		tiles = []rr{{4, 4, 11, 11, 1}, {17, 4, 11, 11, 1}, {4, 17, 11, 11, 1}, {17, 17, 11, 11, 1}}
	}
	inside := func(r rr, x, y float64) bool {
		x, y = x/k, y/k
		if x < r.x || y < r.y || x > r.x+r.w || y > r.y+r.h {
			return false
		}
		cx := math.Max(r.x+r.r, math.Min(x, r.x+r.w-r.r))
		cy := math.Max(r.y+r.r, math.Min(y, r.y+r.h-r.r))
		return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r.r*r.r
	}
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var a, white float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x, y := float64(px)+(float64(sx)+0.5)/ss, float64(py)+(float64(sy)+0.5)/ss
					if !inside(outer, x, y) {
						continue
					}
					a++
					for _, t := range tiles {
						if inside(t, x, y) {
							white++
							break
						}
					}
				}
			}
			if a == 0 {
				continue
			}
			f := white / a // 白色占这个像素的比例
			mix := func(c uint8) uint8 { return uint8(math.Round(float64(c)*(1-f) + 255*f)) }
			img.SetNRGBA(px, py, color.NRGBA{mix(bg.R), mix(bg.G), mix(bg.B), uint8(math.Round(255 * a / (ss * ss)))})
		}
	}
	return img
}

// MakeICO 画默认图标并编码成 .ico（256 用 PNG，其它用 32 位 BMP，兼容性最好）。
func MakeICO() ([]byte, error) {
	var imgs []IconImage
	for _, s := range IconSizes {
		im := DrawIcon(s)
		var data []byte
		if s >= 256 {
			var b bytes.Buffer
			if err := png.Encode(&b, im); err != nil {
				return nil, err
			}
			data = b.Bytes()
		} else {
			data = dib(im)
		}
		imgs = append(imgs, IconImage{W: s, H: s, BitCount: 32, Data: data})
	}
	return EncodeICO(imgs), nil
}

// dib 把图片编码成 .ico 里用的 32 位 BMP（高度写两倍：下半是 AND 遮罩）。
func dib(im *image.NRGBA) []byte {
	w, h := im.Bounds().Dx(), im.Bounds().Dy()
	maskRow := ((w + 31) / 32) * 4
	var b bytes.Buffer
	le := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	le(uint32(40))
	le(int32(w))
	le(int32(h * 2))
	le(uint16(1))
	le(uint16(32))
	le(uint32(0))
	le(uint32(w*h*4 + maskRow*h))
	le(int32(0))
	le(int32(0))
	le(uint32(0))
	le(uint32(0))
	for y := h - 1; y >= 0; y-- { // BMP 从最下面一行开始
		for x := 0; x < w; x++ {
			c := im.NRGBAAt(x, y)
			b.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	for y := h - 1; y >= 0; y-- { // AND 遮罩：透明的地方为 1
		row := make([]byte, maskRow)
		for x := 0; x < w; x++ {
			if im.NRGBAAt(x, y).A == 0 {
				row[x/8] |= 0x80 >> (x % 8)
			}
		}
		b.Write(row)
	}
	return b.Bytes()
}

// EncodeICO 把几个尺寸组成 .ico 文件。
func EncodeICO(imgs []IconImage) []byte {
	var b bytes.Buffer
	le := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	le(uint16(0))
	le(uint16(1))
	le(uint16(len(imgs)))
	off := 6 + 16*len(imgs)
	for _, im := range imgs {
		b.WriteByte(byte(im.W % 256))
		b.WriteByte(byte(im.H % 256))
		b.WriteByte(0)
		b.WriteByte(0)
		le(uint16(1))
		le(im.BitCount)
		le(uint32(len(im.Data)))
		le(uint32(off))
		off += len(im.Data)
	}
	for _, im := range imgs {
		b.Write(im.Data)
	}
	return b.Bytes()
}

// ReadICO 读 .ico 文件（维护者可以把 build/windows/app.ico 换成自己设计的图标）。
func ReadICO(b []byte) ([]IconImage, error) {
	if len(b) < 6 || binary.LittleEndian.Uint16(b[0:]) != 0 || binary.LittleEndian.Uint16(b[2:]) != 1 {
		return nil, errors.New("不是 .ico 文件")
	}
	n := int(binary.LittleEndian.Uint16(b[4:]))
	if n == 0 || len(b) < 6+16*n {
		return nil, errors.New(".ico 里没有图片")
	}
	var out []IconImage
	for i := 0; i < n; i++ {
		e := b[6+16*i:]
		w, h := int(e[0]), int(e[1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		size := int(binary.LittleEndian.Uint32(e[8:]))
		off := int(binary.LittleEndian.Uint32(e[12:]))
		if off < 0 || size <= 0 || off+size > len(b) {
			return nil, fmt.Errorf(".ico 第 %d 张图片的位置不对", i+1)
		}
		out = append(out, IconImage{W: w, H: h, BitCount: binary.LittleEndian.Uint16(e[6:]), Data: b[off : off+size]})
	}
	return out, nil
}

// ---------- 版本信息 ----------

func utf16z(s string) []byte {
	u := utf16.Encode([]rune(s + "\x00"))
	b := make([]byte, 2*len(u))
	for i, v := range u {
		binary.LittleEndian.PutUint16(b[2*i:], v)
	}
	return b
}

func pad4(b []byte) []byte {
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

// node 组一个版本信息的区块：wLength、wValueLength、wType、键名、值、子区块（都对齐 4 字节）
func node(key string, typ uint16, value []byte, valueLen uint16, children ...[]byte) []byte {
	b := make([]byte, 6)
	b = append(b, utf16z(key)...)
	b = pad4(b)
	b = append(b, value...)
	for _, c := range children {
		b = pad4(b)
		b = append(b, c...)
	}
	binary.LittleEndian.PutUint16(b[0:], uint16(len(b)))
	binary.LittleEndian.PutUint16(b[2:], valueLen)
	binary.LittleEndian.PutUint16(b[4:], typ)
	return b
}

// ParseVersion 把「1.0.1」变成 4 个数字。
func ParseVersion(v string) ([4]uint16, error) {
	var out [4]uint16
	parts := strings.Split(strings.TrimSpace(v), ".")
	if len(parts) < 1 || len(parts) > 4 {
		return out, fmt.Errorf("版本号格式不对：%q", v)
	}
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return out, fmt.Errorf("版本号格式不对：%q", v)
		}
		out[i] = uint16(n)
	}
	return out, nil
}

// VersionInfo 产生 RT_VERSION 的内容。
func VersionInfo(in Info) ([]byte, error) {
	v, err := ParseVersion(in.Version)
	if err != nil {
		return nil, err
	}
	fixed := make([]byte, 52)
	put := func(i int, x uint32) { binary.LittleEndian.PutUint32(fixed[4*i:], x) }
	ms, ls := uint32(v[0])<<16|uint32(v[1]), uint32(v[2])<<16|uint32(v[3])
	put(0, 0xFEEF04BD) // 签名
	put(1, 0x00010000)
	put(2, ms)
	put(3, ls)
	put(4, ms)
	put(5, ls)
	put(6, 0x3F)
	put(7, 0)
	put(8, 0x40004) // VOS_NT_WINDOWS32
	put(9, 1)       // VFT_APP
	str := func(k, val string) []byte {
		return node(k, 1, utf16z(val), uint16(len(utf16.Encode([]rune(val)))+1))
	}
	table := node("080404B0", 1, nil, 0,
		str("CompanyName", in.Company),
		str("FileDescription", in.Description),
		str("FileVersion", in.Version),
		str("InternalName", strings.TrimSuffix(in.FileName, ".exe")),
		str("OriginalFilename", in.FileName),
		str("ProductName", in.Product),
		str("ProductVersion", in.Version),
	)
	trans := make([]byte, 4)
	binary.LittleEndian.PutUint16(trans[0:], langZhCN)
	binary.LittleEndian.PutUint16(trans[2:], 1200) // Unicode
	return node("VS_VERSION_INFO", 0, fixed, 52,
		node("StringFileInfo", 1, nil, 0, table),
		node("VarFileInfo", 1, nil, 0, node("Translation", 0, trans, 4)),
	), nil
}

// Manifest 让对话框用新式外观、在高分辨率屏幕上不模糊，并声明支持 Windows 10 / 11。
func Manifest(in Info) ([]byte, error) {
	v, err := ParseVersion(in.Version)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0" xmlns:asmv3="urn:schemas-microsoft-com:asm.v3">
  <assemblyIdentity type="win32" name="TJHS.%s" version="%d.%d.%d.%d" processorArchitecture="amd64"/>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
  <asmv3:application>
    <asmv3:windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2</dpiAwareness>
    </asmv3:windowsSettings>
  </asmv3:application>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
</assembly>
`, strings.TrimSuffix(in.FileName, ".exe"), v[0], v[1], v[2], v[3])), nil
}

// ---------- COFF .syso ----------

type resItem struct {
	typ, id uint32
	data    []byte
}

// Syso 产生包含图标、版本信息、manifest 的 COFF 目标文件。
func Syso(icons []IconImage, in Info) ([]byte, error) {
	if len(icons) == 0 {
		return nil, errors.New("没有图标")
	}
	var items []resItem
	group := new(bytes.Buffer)
	le := func(v any) { binary.Write(group, binary.LittleEndian, v) }
	le(uint16(0))
	le(uint16(1))
	le(uint16(len(icons)))
	for i, im := range icons {
		items = append(items, resItem{rtIcon, uint32(i + 1), im.Data})
		group.WriteByte(byte(im.W % 256))
		group.WriteByte(byte(im.H % 256))
		group.WriteByte(0)
		group.WriteByte(0)
		le(uint16(1))
		le(im.BitCount)
		le(uint32(len(im.Data)))
		le(uint16(i + 1))
	}
	items = append(items, resItem{rtGroupIcon, 1, group.Bytes()})
	ver, err := VersionInfo(in)
	if err != nil {
		return nil, err
	}
	items = append(items, resItem{rtVersion, 1, ver})
	man, err := Manifest(in)
	if err != nil {
		return nil, err
	}
	items = append(items, resItem{rtManifest, 1, man})
	rsrc, relocs := buildRsrc(items)
	return coff(rsrc, relocs), nil
}

// buildRsrc 排出资源目录树（类型 → 编号 → 语言 → 数据），回传 .rsrc 的内容和要重定位的位置
func buildRsrc(items []resItem) ([]byte, []uint32) {
	// 依类型分组（items 已经照类型、编号排好）
	var types []uint32
	byType := map[uint32][]resItem{}
	for _, it := range items {
		if _, ok := byType[it.typ]; !ok {
			types = append(types, it.typ)
		}
		byType[it.typ] = append(byType[it.typ], it)
	}
	dirSize := func(n int) int { return 16 + 8*n }
	// 先算每一层的位置
	root := 0
	off := dirSize(len(types))
	typeDir := map[uint32]int{}
	for _, t := range types {
		typeDir[t] = off
		off += dirSize(len(byType[t]))
	}
	type key struct{ t, id uint32 }
	langDir := map[key]int{}
	for _, t := range types {
		for _, it := range byType[t] {
			langDir[key{t, it.id}] = off
			off += dirSize(1)
		}
	}
	dataEntry := map[key]int{}
	for _, t := range types {
		for _, it := range byType[t] {
			dataEntry[key{t, it.id}] = off
			off += 16
		}
	}
	dataAt := map[key]int{}
	for _, t := range types {
		for _, it := range byType[t] {
			off = (off + 7) &^ 7
			dataAt[key{t, it.id}] = off
			off += len(it.data)
		}
	}
	b := make([]byte, off)
	dir := func(at, n int) {
		binary.LittleEndian.PutUint16(b[at+14:], uint16(n)) // 全部用编号（没有名称）
	}
	entry := func(at, i int, id uint32, target int, sub bool) {
		p := at + 16 + 8*i
		binary.LittleEndian.PutUint32(b[p:], id)
		v := uint32(target)
		if sub {
			v |= 0x80000000
		}
		binary.LittleEndian.PutUint32(b[p+4:], v)
	}
	var relocs []uint32
	dir(root, len(types))
	for i, t := range types {
		entry(root, i, t, typeDir[t], true)
		dir(typeDir[t], len(byType[t]))
		for j, it := range byType[t] {
			k := key{t, it.id}
			entry(typeDir[t], j, it.id, langDir[k], true)
			dir(langDir[k], 1)
			entry(langDir[k], 0, langZhCN, dataEntry[k], false)
			de := dataEntry[k]
			binary.LittleEndian.PutUint32(b[de:], uint32(dataAt[k])) // 链接时会加上 .rsrc 的 RVA
			binary.LittleEndian.PutUint32(b[de+4:], uint32(len(it.data)))
			relocs = append(relocs, uint32(de))
			copy(b[dataAt[k]:], it.data)
		}
	}
	return b, relocs
}

// coff 把 .rsrc 包成 AMD64 的 COFF 目标文件（go build 会自动链接同文件夹的 .syso）
func coff(rsrc []byte, relocs []uint32) []byte {
	const hdr, sec = 20, 40
	rawAt := hdr + sec
	relAt := rawAt + len(rsrc)
	symAt := relAt + 10*len(relocs)
	var b bytes.Buffer
	le := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	// 文件头
	le(uint16(0x8664)) // AMD64
	le(uint16(1))      // 1 个区段
	le(uint32(0))
	le(uint32(symAt))
	le(uint32(1)) // 1 个符号
	le(uint16(0))
	le(uint16(0))
	// 区段头
	b.WriteString(".rsrc\x00\x00\x00")
	le(uint32(0))
	le(uint32(0))
	le(uint32(len(rsrc)))
	le(uint32(rawAt))
	le(uint32(relAt))
	le(uint32(0))
	le(uint16(len(relocs)))
	le(uint16(0))
	le(uint32(0x40000040)) // 已初始化资料、可读
	b.Write(rsrc)
	for _, r := range relocs {
		le(r)
		le(uint32(0)) // 第 0 个符号（.rsrc 区段）
		le(uint16(3)) // IMAGE_REL_AMD64_ADDR32NB
	}
	// 符号表：.rsrc 区段符号
	b.WriteString(".rsrc\x00\x00\x00")
	le(uint32(0))
	le(int16(1))
	le(uint16(0))
	b.WriteByte(3) // IMAGE_SYM_CLASS_STATIC
	b.WriteByte(0)
	le(uint32(4)) // 字串表（空的）
	return b.Bytes()
}
