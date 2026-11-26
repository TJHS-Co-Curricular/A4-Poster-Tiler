package winres

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func TestICORoundTrip(t *testing.T) {
	b, err := MakeICO()
	if err != nil {
		t.Fatal(err)
	}
	imgs, err := ReadICO(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != len(IconSizes) {
		t.Fatalf("尺寸数量：%d", len(imgs))
	}
	for i, im := range imgs {
		if im.W != IconSizes[i] || im.H != IconSizes[i] || im.BitCount != 32 {
			t.Errorf("第 %d 张：%d×%d %d 位", i, im.W, im.H, im.BitCount)
		}
	}
	if !bytes.HasPrefix(imgs[len(imgs)-1].Data, []byte("\x89PNG")) {
		t.Error("256 要用 PNG")
	}
	if binary.LittleEndian.Uint32(imgs[0].Data) != 40 {
		t.Error("小尺寸要用 BMP（BITMAPINFOHEADER = 40）")
	}
	if _, err := ReadICO([]byte("not an icon")); err == nil {
		t.Error("不是 .ico 要报错")
	}
}

func TestDrawIconCorners(t *testing.T) {
	im := DrawIcon(64)
	if im.NRGBAAt(0, 0).A != 0 {
		t.Error("圆角外面要透明")
	}
	if c := im.NRGBAAt(32, 2); c.A != 255 || c.G < 0x80 || c.R > 0x40 {
		t.Errorf("边上要是青绿色：%v", c)
	}
	if c := im.NRGBAAt(20, 22); c.R != 255 || c.G != 255 || c.B != 255 {
		t.Errorf("纸的中间要是白色：%v", c)
	}
}

func TestVersionInfo(t *testing.T) {
	b, err := VersionInfo(Info{Version: "1.2.3", Company: "测试甲学校", Description: "A4 拼贴海报", Product: "A4 拼贴海报", FileName: "A4-Poster-Tiler.exe"})
	if err != nil {
		t.Fatal(err)
	}
	if int(binary.LittleEndian.Uint16(b)) != len(b) {
		t.Fatal("wLength 要等于整个长度")
	}
	sig := bytes.Index(b, []byte{0xBD, 0x04, 0xEF, 0xFE})
	if sig < 0 || sig%4 != 0 {
		t.Fatal("找不到 VS_FIXEDFILEINFO 签名（或没对齐）")
	}
	if ms, ls := binary.LittleEndian.Uint32(b[sig+8:]), binary.LittleEndian.Uint32(b[sig+12:]); ms != 1<<16|2 || ls != 3<<16 {
		t.Fatalf("版本号：%x %x", ms, ls)
	}
	for _, s := range []string{"1.2.3", "测试甲学校", "A4-Poster-Tiler.exe", "080404B0"} {
		if !bytes.Contains(b, utf16le(s)) {
			t.Errorf("少了 %s", s)
		}
	}
	if _, err := VersionInfo(Info{Version: "abc"}); err == nil {
		t.Error("版本号写错要报错")
	}
}

func utf16le(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}

// .syso 要是合法的 COFF：一个 .rsrc 区段，每个资源一个重定位，资源目录照类型排好
func TestSyso(t *testing.T) {
	ico, _ := MakeICO()
	icons, _ := ReadICO(ico)
	b, err := Syso(icons, Info{Version: "1.0.1", FileName: "A4-Poster-Tiler.exe"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := pe.NewFile(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("不是合法的 COFF：%v", err)
	}
	if f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || len(f.Sections) != 1 || f.Sections[0].Name != ".rsrc" {
		t.Fatalf("区段不对：%v %d", f.Machine, len(f.Sections))
	}
	s := f.Sections[0]
	want := len(icons) + 3 // 每张图标 + 图标组 + 版本 + manifest
	if len(s.Relocs) != want {
		t.Fatalf("重定位数量 %d，应该 %d", len(s.Relocs), want)
	}
	data, _ := s.Data()
	if n := binary.LittleEndian.Uint16(data[14:]); n != 4 {
		t.Fatalf("根目录应该有 4 种资源：%d", n)
	}
	var types []uint32
	for i := 0; i < 4; i++ {
		types = append(types, binary.LittleEndian.Uint32(data[16+8*i:]))
	}
	if types[0] != rtIcon || types[1] != rtGroupIcon || types[2] != rtVersion || types[3] != rtManifest {
		t.Fatalf("资源类型要由小到大：%v", types)
	}
	for _, r := range s.Relocs {
		if r.Type != 3 || r.SymbolTableIndex != 0 {
			t.Fatalf("重定位类型不对：%+v", r)
		}
		off := binary.LittleEndian.Uint32(data[r.VirtualAddress:])
		size := binary.LittleEndian.Uint32(data[r.VirtualAddress+4:])
		if int(off+size) > len(data) || off%8 != 0 {
			t.Fatalf("资源数据位置不对：%d + %d", off, size)
		}
	}
}
