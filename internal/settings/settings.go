// Package settings 读取运行时文件夹里的 config/config.ini。
//
//   - 文件不存在：用打包在 exe 里的默认内容建立（带 BOM、CRLF）。
//   - 缺键或缺区段：只在文件末尾补上默认值和注释，绝不改动老师已经写的值（只补不改）。
//   - 值写错：退回默认值，Issues 里记下第几行写错，启动横幅用 [注意] 显示，不闪退。
package settings

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Settings 是 config.ini 的内容。
type Settings struct {
	Access       string  // [server] access：local / lan
	Port         int     // [server] port
	PortFallback bool    // [server] port_fallback
	OpenBrowser  bool    // [server] open_browser
	AppWindow    bool    // [server] app_window：用 Edge / Chrome 的独立窗口打开
	LANAllowEdit bool    // [server] lan_allow_edit
	OutputFolder string  // [paths] output_folder
	LogLevel     string  // [log] level
	LogKeepDays  int     // [log] keep_days
	MarginMM     float64 // [tile] margin_mm
	OverlapMM    float64 // [tile] overlap_mm

	Path     string   // 设置文件完整路径
	Created  bool     // 这次运行才建立的
	Appended []string // 这次补上的键（section.key）
	Issues   []string // 写错、被略过的行
}

// LAN 回传是否开放给局域网。
func (s Settings) LAN() bool { return s.Access == "lan" }

// Defaults 回传程序内建的默认值（和 config/config.ini 一致）。
func Defaults() Settings {
	return Settings{
		Access: "local", Port: 5030, PortFallback: true, OpenBrowser: true, AppWindow: true, LANAllowEdit: false,
		OutputFolder: "output", LogLevel: "info", LogKeepDays: 30, MarginMM: 8, OverlapMM: 10,
	}
}

// Load 读取 path；不存在时用 defaults 建立；缺键时在末尾补上。
func Load(path string, defaults []byte) (Settings, error) {
	s := Defaults()
	s.Path = path

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if werr := os.WriteFile(path, withBOM(toCRLF(defaults)), 0o644); werr != nil {
			return s, fmt.Errorf("无法建立设置文件 %s：%w", path, werr)
		}
		s.Created = true
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("无法读取设置文件 %s：%w（改用默认设置）", path, err)
	}
	seen := s.apply(b)

	if add, keys := missingBlock(defaults, seen); len(keys) > 0 {
		if err := appendFile(path, b, add); err != nil {
			s.Issues = append(s.Issues, fmt.Sprintf("缺少 %s，无法自动补上（%v），先用默认值", strings.Join(keys, "、"), err))
		} else {
			s.Appended = keys
		}
	}
	return s, nil
}

type entry struct {
	section, key string
	comments     []string
	line         string
}

// 解析默认设置文件：每个键和它上面的注释
func templateEntries(tpl []byte) []entry {
	var out []entry
	var pending []string
	section := ""
	for _, raw := range strings.Split(decodeText(tpl), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
			pending = nil
		case strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#"):
			pending = append(pending, line)
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			pending = nil
		default:
			k, _, ok := strings.Cut(line, "=")
			if ok {
				out = append(out, entry{section, strings.ToLower(strings.TrimSpace(k)), pending, line})
			}
			pending = nil
		}
	}
	return out
}

// missingBlock 回传要补在文件末尾的文字，以及补了哪些键。
func missingBlock(tpl []byte, seen map[string]bool) (string, []string) {
	var sb strings.Builder
	var keys []string
	cur := ""
	for _, e := range templateEntries(tpl) {
		full := e.section + "." + e.key
		if seen[full] {
			continue
		}
		if cur != e.section {
			// 区段已经存在也再写一次标题：只在末尾追加，不动老师原来的内容
			sb.WriteString("\n[" + e.section + "]\n")
			cur = e.section
		}
		sb.WriteString("; （程序自动补上的默认值）\n")
		for _, c := range e.comments {
			sb.WriteString(c + "\n")
		}
		sb.WriteString(e.line + "\n")
		keys = append(keys, full)
	}
	return sb.String(), keys
}

func appendFile(path string, old []byte, add string) error {
	if len(old) >= 2 && (old[0] == 0xFF && old[1] == 0xFE || old[0] == 0xFE && old[1] == 0xFF) {
		return errors.New("文件是 UTF-16 编码，请用记事本另存为 UTF-8")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	prefix := ""
	if len(old) > 0 && !bytes.HasSuffix(old, []byte("\n")) {
		prefix = "\n"
	}
	_, err = f.Write(toCRLF([]byte(prefix + add)))
	return err
}

// apply 读取 b 里的值；回传出现过的键（section.key）和区段（[section]）。
func (s *Settings) apply(b []byte) map[string]bool {
	seen := map[string]bool{}
	section := ""
	def := Defaults()
	bad := func(line int, key, msg, fallback string) {
		s.Issues = append(s.Issues, fmt.Sprintf("config.ini 第 %d 行 %s %s，改用默认 %s", line, key, msg, fallback))
	}
	for i, raw := range strings.Split(decodeText(b), "\n") {
		n := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			seen["["+section+"]"] = true
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			s.Issues = append(s.Issues, fmt.Sprintf("config.ini 第 %d 行缺少「=」，已略过", n))
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		key := section + "." + k
		seen[key] = true
		switch key {
		case "server.access":
			switch strings.ToLower(v) {
			case "local", "lan":
				s.Access = strings.ToLower(v)
			default:
				bad(n, k, "只能是 local 或 lan", def.Access)
				s.Access = def.Access
			}
		case "server.port":
			if p, err := strconv.Atoi(v); err == nil && p >= 1024 && p <= 65535 {
				s.Port = p
			} else {
				bad(n, k, "要是 1024 ~ 65535 的数字", strconv.Itoa(def.Port))
				s.Port = def.Port
			}
		case "server.port_fallback":
			s.PortFallback = s.boolVal(n, k, v, def.PortFallback)
		case "server.open_browser":
			s.OpenBrowser = s.boolVal(n, k, v, def.OpenBrowser)
		case "server.app_window":
			s.AppWindow = s.boolVal(n, k, v, def.AppWindow)
		case "server.lan_allow_edit":
			s.LANAllowEdit = s.boolVal(n, k, v, def.LANAllowEdit)
		case "paths.output_folder":
			if v == "" {
				bad(n, k, "是空的", def.OutputFolder)
				s.OutputFolder = def.OutputFolder
			} else {
				s.OutputFolder = v
			}
		case "paths.result_folder":
			// 标准键，这个程序不用 Result/，写了也不报错
		case "log.level":
			switch strings.ToLower(v) {
			case "debug", "info", "warning", "warn", "error":
				s.LogLevel = strings.ToLower(v)
			default:
				bad(n, k, "只能是 debug / info / warning / error", def.LogLevel)
				s.LogLevel = def.LogLevel
			}
		case "log.keep_days":
			if d, err := strconv.Atoi(v); err == nil && d >= 0 {
				s.LogKeepDays = d
			} else {
				bad(n, k, "要是 0 以上的整数", strconv.Itoa(def.LogKeepDays))
				s.LogKeepDays = def.LogKeepDays
			}
		case "tile.margin_mm":
			s.MarginMM = s.numVal(n, k, v, 3, 30, def.MarginMM)
		case "tile.overlap_mm":
			s.OverlapMM = s.numVal(n, k, v, 3, 50, def.OverlapMM)
		default:
			s.Issues = append(s.Issues, fmt.Sprintf("config.ini 第 %d 行「%s」不认识，已略过", n, key))
		}
	}
	return seen
}

func (s *Settings) boolVal(line int, key, v string, def bool) bool {
	if b, ok := ParseBool(v); ok {
		return b
	}
	d := map[bool]string{true: "yes", false: "no"}[def]
	s.Issues = append(s.Issues, fmt.Sprintf("config.ini 第 %d 行 %s 只能是 yes 或 no，改用默认 %s", line, key, d))
	return def
}

func (s *Settings) numVal(line int, key, v string, lo, hi, def float64) float64 {
	f, err := strconv.ParseFloat(v, 64)
	if err == nil && f >= lo && f <= hi {
		return f
	}
	s.Issues = append(s.Issues, fmt.Sprintf("config.ini 第 %d 行 %s 要是 %g ~ %g 的数字，改用默认 %g", line, key, lo, hi, def))
	return def
}

// ParseBool 接受 yes/no、true/false、1/0、on/off、是/否（不分大小写）。
func ParseBool(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "true", "1", "on", "是":
		return true, true
	case "no", "false", "0", "off", "否":
		return false, true
	}
	return false, false
}

// 记事本可能存成 UTF-8（含 BOM）或 UTF-16（「Unicode」），都转成一般字串
func decodeText(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		b = b[3:]
	case len(b) >= 2 && (b[0] == 0xFF && b[1] == 0xFE || b[0] == 0xFE && b[1] == 0xFF):
		le := b[0] == 0xFF
		b = b[2:]
		u := make([]uint16, len(b)/2)
		for i := range u {
			if le {
				u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
			} else {
				u[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
			}
		}
		return strings.ReplaceAll(string(utf16.Decode(u)), "\r", "")
	}
	return strings.ReplaceAll(string(b), "\r", "")
}

func toCRLF(b []byte) []byte {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	return []byte(strings.ReplaceAll(s, "\n", "\r\n"))
}

func withBOM(b []byte) []byte {
	if bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		return b
	}
	return append([]byte{0xEF, 0xBB, 0xBF}, b...)
}
