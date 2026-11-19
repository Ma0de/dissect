// Package renpy: parse.go —— 对 unrpyc 反编译出的 .rpy 文本做行级扫描，
// 提取：角色定义 / store 字段 / 类字段 / screen 参数 / 台词 / 选项 / label 流程图 /
// image 定义与引用。该解析器不绑定具体游戏，对反编译后的任意 Ren'Py 脚本有效。
package renpy

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"dissect/internal/model"
)

type Parser struct {
	Model *model.GameModel
	// image 名(长名含空格) → 归档路径
	images map[string]string
	// label → 定义位置
	labels map[string]model.FlowNode
	// 边（用于去重排序）
	edges    []model.FlowEdge
	edgeSeen map[string]bool
	// 台词顺序
	dlgID    int
	choiceID int
	// speaker → count
	speakerCnt map[string]int
	// image 引用位置 file → [script位置]
	pendingUsage map[string][]string
	// 上一句说话人（extend 用）
	lastSpeaker string
	// 已被 choice 捕获的 jump/call 行（file:line），主循环不再重复计边
	suppressed map[string]bool
	// 运行时状态
	labelStack []string
	// auto_voice 文件名模板与已生成的翻译 id
	autoVoice string
	voiceIDs  map[string]bool
}

func NewParser() *Parser {
	return &Parser{
		Model:      &model.GameModel{},
		suppressed: map[string]bool{},
		images:     map[string]string{},
		labels:     map[string]model.FlowNode{},
		edgeSeen:   map[string]bool{},
		speakerCnt: map[string]int{},
		voiceIDs:   map[string]bool{},
	}
}

var (
	reLabel            = regexp.MustCompile(`^label\s+([\w.]+)\s*(\((.*)\))?\s*:$`)
	reJump             = regexp.MustCompile(`^(?:- )?jump\s+(\w+(?:\.\w+)*)(?:\s+expression\s+(\S+))?(?:\s+if\s+(.+))?$`)
	reCall             = regexp.MustCompile(`^(?:- )?call\s+(?:(screen)\s+)?(\w+(?:\.\w+)*)(\((.*)\))?(?:\s+from\s+\S+)?(?:\s+if\s+(.+))?$`)
	reMenu             = regexp.MustCompile(`^menu(?:\s+\w+)?:`)
	reDefine           = regexp.MustCompile(`^(define|default)\s+([\w.]+)\s*=\s*(.+)$`)
	reAutoVoice        = regexp.MustCompile(`(?m)^\s*define\s+config\.auto_voice\s*=\s*["']([^"']+)["']`)
	reCharDef          = regexp.MustCompile(`^(?:define|default)\s+(\w+)\s*=\s*Character\s*\((.*)\)\s*$`)
	reImageDef         = regexp.MustCompile(`^image\s+((?:[\w.]+\s*)+?)=\s*(.+)$`)
	reImageBlock       = regexp.MustCompile(`^image\s+((?:[\w.]+)(?:\s+[\w.]+)*)\s*:$`)
	rePersistentAssign = regexp.MustCompile(`^\$\s*persistent\.([\w.]+)\s*=\s*(.+)$`)
	reScreen           = regexp.MustCompile(`^screen\s+(\w+)\s*(\((.*)\))?\s*:$`)
	reClass            = regexp.MustCompile(`^\s*class\s+(\w+)\s*(\(.*\))?\s*:$`)
	reSelfAttr         = regexp.MustCompile(`^\s*self\.(\w+)\s*=(.*)$`)
	reClassAttr        = regexp.MustCompile(`^(\s*)(\w+)\s*=\s*(.+)$`)
	rePythonBlock      = regexp.MustCompile(`^(?:init\s+)?python\s*(?:\S*)?\s*:$`)
	reAssign           = regexp.MustCompile(`^\$\s*(.+)$`)
	reSay              = regexp.MustCompile(`^(.+?)\s+("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')(?:\s+if\s+(.+?))?(?:\s+with\s+(.+?))?(?:\s+nointeract)?$`)
	reQuoted           = regexp.MustCompile(`^(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')(?:\s+if\s+(.+?))?(?:\s+with\s+(.+?))?(?:\s+nointeract)?$`)
	reScene            = regexp.MustCompile(`^(scene|show|hide)\s+(.+?)(?:\s+with\s+\S+|\s+at\s+.+|\s+behind\s+.+|\s+zorder\s+\d+|\s+if\s+.+)?$`)
	reOld              = regexp.MustCompile(`^old\s+"(.*)"$`)
	reTranslateStr     = regexp.MustCompile(`^\s*(?:old\s+)?("(?:[^"\\]|\\.)*")\s*$`)
	reReturn           = regexp.MustCompile(`^return(?:\s+.*)?$`)
	rePass             = regexp.MustCompile(`^(pass|pause)(\s+.*)?$`)
)

// statement 关键字（非台词）
var stmtKeywords = map[string]bool{
	"scene": true, "show": true, "hide": true, "with": true, "window": true,
	"play": true, "queue": true, "stop": true, "voice": true, "jump": true,
	"call": true, "return": true, "if": true, "elif": true, "else": true,
	"while": true, "for": true, "menu": true, "pause": true, "extend": true,
	"label": true, "default": true, "define": true, "image": true, "transform": true,
	"style": true, "screen": true, "use": true, "python": true, "init": true,
	"translate": true, "pass": true, "nvl": true, "nvls": true, "camera": true,
	"new": true, "strings": true, "layers": true, "rpy": true, "nointeract": true,
	// screen 语言属性 / 语句首词，防误判为台词说话人
	"add": true, "text": true, "textbutton": true, "button": true, "old": true,
	"idle": true, "hover": true, "selected_idle": true, "selected_hover": true,
	"sensitive": true, "background": true, "modal": true, "zorder": true,
	"frame": true, "vbox": true, "hbox": true, "grid": true, "fixed": true,
	"bar": true, "vbar": true, "imagebutton": true, "hotspot": true, "hotbar": true,
	"key": true, "timer": true, "drag": true, "draggroup": true, "viewport": true,
	"vpgrid": true, "side": true, "null": true, "dismiss": true, "imagemap": true,
	"layer": true, "color": true, "gui": true, "persistent": true, "renpy": true,
	"config": true,
}

// ParseFiles 解析一组 rpy 文本（path → 内容）。
func (p *Parser) ParseFiles(files map[string]string) {
	// 先扫所有文件收集 label / image / define（第一遍），再扫台词（第二遍可选）
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		if m := reAutoVoice.FindStringSubmatch(files[path]); m != nil {
			p.autoVoice = m[1]
		}
	}
	for _, path := range paths {
		p.scanFile(path, files[path])
	}
	// 回填统计
	p.finalize()
}

type lineTok struct {
	indent int
	text   string
	num    int
}

func splitLines(content string) []lineTok {
	out := []lineTok{}
	for i, raw := range strings.Split(content, "\n") {
		raw = strings.TrimRight(raw, "\r")
		trimmed := strings.TrimLeft(raw, " \t")
		if strings.TrimSpace(trimmed) == "" || strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "#!") {
			// 保留 # 注释? 跳过纯注释行
			continue
		}
		indent := len(raw) - len(trimmed)
		indent = indent / 4 // unrpyc 输出 4 空格缩进; tab 容错
		out = append(out, lineTok{indent, strings.TrimRight(trimmed, " \t"), i + 1})
	}
	return out
}

// scanFile 单文件主扫描：块感知的行状态机。
func (p *Parser) scanFile(path, content string) {
	// tl 翻译目录与原脚本重复，跳过（在 Notes 里记录）
	if strings.Contains(path, "/tl/") || strings.Contains(path, "\\tl\\") {
		p.Model.Notes = append(p.Model.Notes, "跳过翻译目录文件: "+path)
		return
	}
	toks := splitLines(content)
	p.Model.ScriptFiles = append(p.Model.ScriptFiles, model.ScriptFile{
		Path: path, Lines: len(toks), Decompiled: true,
	})
	curLabel := ""
	// python 块 / class 上下文
	inClass := ""
	classIndent := 0

	for idx := 0; idx < len(toks); idx++ {
		t := toks[idx]
		line := t.text
		lineNo := t.num

		// —— class / python 块内 ——
		if inClass != "" && t.indent > classIndent {
			if m := reSelfAttr.FindStringSubmatch(line); m != nil {
				p.addField("class:"+inClass, inClass, m[1], guessType(m[2]),
					strings.TrimSpace(m[2]), "self", path, lineNo)
			}
			continue
		}
		if inClass != "" && t.indent <= classIndent {
			inClass = ""
		}

		// —— 顶层语句分发 ——
		switch {
		case reLabel.MatchString(line):
			m := reLabel.FindStringSubmatch(line)
			curLabel = m[1]
			node := model.FlowNode{ID: "L:" + curLabel, Label: curLabel, Kind: "label", File: path, Line: lineNo}
			if _, dup := p.labels[node.ID]; !dup {
				p.labels[node.ID] = node
			}

		case reJump.MatchString(line):
			if p.suppressed[fmt.Sprintf("%s:%d", path, lineNo)] {
				continue
			}
			m := reJump.FindStringSubmatch(line)
			target, expr, cond := m[1], m[2], m[3]
			if expr != "" {
				target = "expression:" + expr
			}
			p.addEdge(curLabel, target, "jump", "", cond, path, lineNo)

		case reCall.MatchString(line):
			if p.suppressed[fmt.Sprintf("%s:%d", path, lineNo)] {
				continue
			}
			m := reCall.FindStringSubmatch(line)
			kind := "call"
			if m[1] == "screen" {
				kind = "call-screen"
				p.ensureScreenNode(m[2])
			}
			p.addEdge(curLabel, m[2], kind, "", m[4], path, lineNo)

		case reMenu.MatchString(line):
			// 收集 menu 直接子级中的所有选项行，并向前扫描每个选项子块的首个 jump/call
			choiceIndent := -1
			firstChoice := -1
			for j := idx + 1; j < len(toks); j++ {
				nt := toks[j]
				if nt.indent <= t.indent {
					break
				}
				if choiceIndent == -1 {
					// menu 块内第一个选项行决定选项缩进（set/caption 行跳过）
					if !isChoiceLine(nt.text) {
						continue
					}
					choiceIndent = nt.indent
				}
				if nt.indent != choiceIndent || !isChoiceLine(nt.text) {
					continue
				}
				if firstChoice == -1 {
					firstChoice = j
				}
				txt, cond := splitChoice(nt.text)
				target, targetKind := "", ""
				k := j + 1
				for k < len(toks) && toks[k].indent > choiceIndent {
					if m := reJump.FindStringSubmatch(toks[k].text); m != nil {
						target, targetKind = m[1], "jump"
					} else if m := reCall.FindStringSubmatch(toks[k].text); m != nil {
						if m[1] == "screen" {
							target, targetKind = m[2], "call-screen"
							p.ensureScreenNode(m[2])
						} else {
							target, targetKind = m[2], "call"
						}
					}
					if target != "" {
						p.suppressed[fmt.Sprintf("%s:%d", path, toks[k].num)] = true
						break
					}
					k++
				}
				p.addChoice(path, nt.num, txt, cond, target, targetKind, curLabel)
			}
			// 主循环从第一个选项行继续，保证选项子块内台词被收集
			if firstChoice >= 0 {
				idx = firstChoice - 1
			}

		case reCharDef.MatchString(line):
			m := reCharDef.FindStringSubmatch(line)
			p.addCharacter(m[1], m[2], path, lineNo)
			p.addField("character:"+m[1], "", m[1], "Character", trimTo(m[2], 120),
				"define", path, lineNo)

		case reDefine.MatchString(line):
			m := reDefine.FindStringSubmatch(line)
			name, val := m[2], strings.TrimSpace(m[3])
			// define 还可能是普通常量/函数/类等
			p.addField("store", "", name, guessType(val), trimTo(val, 160), m[1], path, lineNo)
			// config.name 游戏标题
			if name == "config.name" {
				p.Model.GameName = unquote(strings.TrimSpace(val))
			} else if name == "config.version" {
				p.Model.EngineMeta = setMeta(p.Model.EngineMeta, "gameVersion", unquote(strings.TrimSpace(val)))
			} else if strings.HasPrefix(name, "gui.") && strings.HasSuffix(name, ".color") {
				// 忽略 gui 颜色等大量噪音
			}

		case reImageDef.MatchString(line):
			m := reImageDef.FindStringSubmatch(line)
			name := strings.TrimSpace(m[1])
			val := strings.TrimSpace(m[2])
			file := extractFilePath(val)
			if file != "" {
				p.images[name] = file
			}

		case reImageBlock.MatchString(line):
			// image name: 块式定义 —— 子块首个字符串字面量为文件路径，整块跳过
			m := reImageBlock.FindStringSubmatch(line)
			name := strings.TrimSpace(m[1])
			j := idx + 1
			for ; j < len(toks); j++ {
				nt := toks[j]
				if nt.indent <= t.indent {
					break
				}
				if isStringLiteral(nt.text) && name != "" {
					if file := extractFilePath(nt.text); file != "" {
						p.images[name] = file
					}
				}
			}
			idx = j - 1

		case rePersistentAssign.MatchString(line):
			m := rePersistentAssign.FindStringSubmatch(line)
			p.addField("persistent", "", "persistent."+m[1], guessType(m[2]),
				strings.TrimSpace(m[2]), "$", path, lineNo)

		case reScreen.MatchString(line):
			m := reScreen.FindStringSubmatch(line)
			name := m[1]
			node := model.FlowNode{ID: "S:" + name, Label: name, Kind: "screen", File: path, Line: lineNo}
			if _, dup := p.labels[node.ID]; !dup {
				p.labels[node.ID] = node
			}
			if m[3] != "" {
				for _, prm := range splitParams(m[3]) {
					flds := strings.SplitN(prm, "=", 2)
					def := ""
					if len(flds) == 2 {
						def = strings.TrimSpace(flds[1])
					}
					p.addField("screen:"+name, name, strings.TrimSpace(flds[0]),
						"param", def, "screen-param", path, lineNo)
				}
			}

		case rePythonBlock.MatchString(line):
			// 接下来的 class 定义需要上下文
			for j := idx + 1; j < len(toks); j++ {
				nt := toks[j]
				if nt.indent <= t.indent {
					break
				}
				if m := reClass.FindStringSubmatch(nt.text); m != nil {
					inClass = m[1]
					classIndent = nt.indent
					// 类内字段扫描到块尾
					k := j + 1
					for k < len(toks) && toks[k].indent > classIndent {
						kt := toks[k]
						if sm := reSelfAttr.FindStringSubmatch(kt.text); sm != nil {
							p.addField("class:"+inClass, inClass, sm[1], guessType(sm[2]),
								strings.TrimSpace(sm[2]), "self", path, kt.num)
						} else if kt.indent == classIndent+1 {
							if sm := reClassAttr.FindStringSubmatch(kt.text); sm != nil &&
								!strings.HasPrefix(sm[2], "def ") {
								p.addField("class:"+inClass, inClass, sm[2], guessType(sm[3]),
									strings.TrimSpace(sm[3]), "class-attr", path, kt.num)
							}
						}
						k++
					}
					idx = k - 1
					break
				}
			}

		case reScene.MatchString(line):
			m := reScene.FindStringSubmatch(line)
			if m[1] != "hide" {
				imgName := strings.TrimSpace(m[2])
				if file := p.resolveImage(imgName); file != "" {
					p.noteUsage(file, fmt.Sprintf("%s:%d", path, lineNo))
				}
			}

		default:
			// —— 台词 / 选项行 / 其他 ——
			p.parseSayOrText(path, lineNo, line, curLabel)
		}
	}
}

// parseSayOrText 处理台词行（含旁白）。
func (p *Parser) parseSayOrText(path string, lineNo int, line, curLabel string) {
	if strings.HasSuffix(line, ":") {
		return // 块头（if/while/...）
	}
	var who, what, cond, with string
	if m := reSay.FindStringSubmatch(line); m != nil && isStringLiteral(m[2]) {
		who = strings.TrimSpace(m[1])
		what = m[2]
		cond = strings.TrimSpace(m[3])
		with = strings.TrimSpace(m[4])
		// 赋值语句不是台词: $ x = "str"
		if strings.HasPrefix(who, "$") || strings.Contains(who, "=") {
			return
		}
		// show/scene/voice/play 等带字符串参数的语句不是台词
		first := who
		if i := strings.IndexAny(first, " \t"); i > 0 {
			first = first[:i]
		}
		if stmtKeywords[first] {
			return
		}
		if first == "" || first == "extend" {
			first = p.lastSpeaker
			if first == "" {
				first = "narrator"
			}
		}
		p.lastSpeaker = first
		p.addDialogue(path, lineNo, first, what, cond, with, curLabel,
			p.voiceIDFor(curLabel, who, unquote(strings.TrimSpace(what))))
		return
	}
	if m := reQuoted.FindStringSubmatch(line); m != nil {
		// 纯引号 = 旁白
		cond = strings.TrimSpace(m[1])
		with = strings.TrimSpace(m[2])
		p.lastSpeaker = "narrator"
		p.addDialogue(path, lineNo, "narrator", strings.TrimSpace(line), cond, with, curLabel,
			p.voiceIDFor(curLabel, "", unquote(strings.TrimSpace(line))))
	}
}

func isStringLiteral(s string) bool {
	return strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "'")
}

func isChoiceLine(s string) bool {
	if !isStringLiteral(strings.TrimSpace(strings.SplitN(s, " if ", 2)[0])) {
		return false
	}
	return strings.HasSuffix(s, ":")
}

func splitChoice(s string) (text, cond string) {
	s = strings.TrimSuffix(s, ":")
	if i := strings.Index(s, " if "); i > 0 {
		return s[:i], strings.TrimSpace(s[i+4:])
	}
	return s, ""
}

func (p *Parser) ensureScreenNode(name string) {
	id := "S:" + name
	if _, ok := p.labels[id]; !ok {
		p.labels[id] = model.FlowNode{ID: id, Label: name, Kind: "screen"}
	}
}

func (p *Parser) resolveImage(imgName string) string {
	// 精确或最长前缀匹配
	if f, ok := p.images[imgName]; ok {
		return f
	}
	best := ""
	bestLen := 0
	for name, f := range p.images {
		if strings.HasPrefix(imgName, name) && len(name) > bestLen {
			best = f
			bestLen = len(name)
		}
	}
	return best
}

func (p *Parser) noteUsage(file, where string) {
	// 存到 asset 引用（assets 尚未构建时先攒着）
	if p.pendingUsage == nil {
		p.pendingUsage = map[string][]string{}
	}
	p.pendingUsage[file] = append(p.pendingUsage[file], where)
}

func (p *Parser) addDialogue(file string, lineNo int, speaker, what, cond, with, label, voiceID string) {
	text := unquote(strings.TrimSpace(what))
	if text == "" {
		return
	}
	p.dlgID++
	d := model.Dialogue{
		ID: p.dlgID, File: file, Line: lineNo, Speaker: speaker, Text: text,
		Plain: stripTags(text), Cond: cond, WithExpr: with, Label: label, VoiceID: voiceID,
	}
	p.Model.Dialogues = append(p.Model.Dialogues, d)
	p.speakerCnt[speaker]++
}

func (p *Parser) voiceIDFor(label, who, text string) string {
	code := ""
	if who != "" {
		code = who + " "
	}
	code += encodeSayString(text)
	sum := md5.Sum([]byte(code + "\r\n"))
	digest := hex.EncodeToString(sum[:])[:8]
	base := digest
	if label != "" {
		base = strings.ReplaceAll(label, ".", "_") + "_" + digest
	}
	id := base
	for i := 1; p.voiceIDs[id]; i++ {
		id = fmt.Sprintf("%s_%d", base, i)
	}
	p.voiceIDs[id] = true
	return id
}

func encodeSayString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '"':
			b.WriteString(`\"`)
		case ' ':
			if i > 0 && s[i-1] == ' ' {
				b.WriteString(`\ `)
			} else {
				b.WriteByte(' ')
			}
		default:
			b.WriteByte(s[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}

func (p *Parser) LinkVoices() {
	paths := map[string]bool{}
	stems := map[string]string{}
	idx := map[string]int{}
	for i, a := range p.Model.Assets {
		if a.Kind != "audio" {
			continue
		}
		paths[a.Path] = true
		idx[a.Path] = i
		stem := strings.TrimSuffix(filepath.Base(a.Path), filepath.Ext(a.Path))
		if _, ok := stems[stem]; !ok {
			stems[stem] = a.Path
		}
	}
	mapped := 0
	for i := range p.Model.Dialogues {
		d := &p.Model.Dialogues[i]
		if d.VoiceID == "" {
			continue
		}
		target := ""
		if p.autoVoice != "" {
			cand := strings.ReplaceAll(p.autoVoice, "{id}", d.VoiceID)
			if paths[cand] {
				target = cand
			}
		}
		if target == "" {
			target = stems[d.VoiceID]
		}
		if target == "" {
			continue
		}
		d.Audio = target
		mapped++
		if ai, ok := idx[target]; ok {
			p.Model.Assets[ai].UsedAt = append(p.Model.Assets[ai].UsedAt,
				fmt.Sprintf("%s:%d", d.File, d.Line))
		}
	}
	p.Model.Stats.VoiceMapped = mapped
}

func (p *Parser) addChoice(file string, lineNo int, text, cond, target, targetKind, label string) {
	p.choiceID++
	p.Model.Choices = append(p.Model.Choices, model.Choice{
		ID: p.choiceID, File: file, Line: lineNo,
		Text: unquote(strings.TrimSpace(text)), Cond: cond,
		Target: target, TargetKind: targetKind, Label: label,
	})
	if target != "" && label != "" {
		kind := "choice"
		if targetKind == "call-screen" {
			kind = "call"
		}
		p.addEdge(label, target, kind, unquote(strings.TrimSpace(text)), cond, file, lineNo)
	}
}

func (p *Parser) addCharacter(tag, args, file string, lineNo int) {
	name := ""
	kind := "normal"
	// Character 第一个参数为显示名
	if s := firstStringArg(args); s != "" {
		name = s
	} else {
		name = tag
		kind = "dynamic"
	}
	if strings.HasPrefix(args, "None") || strings.HasPrefix(args, "null") {
		name = ""
		kind = "narrator"
	}
	for _, c := range p.Model.Characters {
		if c.Tag == tag {
			return
		}
	}
	p.Model.Characters = append(p.Model.Characters, model.Character{
		Tag: tag, Name: name, Kind: kind,
		Sources: []string{fmt.Sprintf("%s:%d", file, lineNo)},
	})
}

func (p *Parser) addField(scope, classOf, name, typ, def, keyword, file string, lineNo int) {
	p.Model.Fields = append(p.Model.Fields, model.Field{
		Name: name, Scope: scope, ClassOf: classOf, Type: typ, Default: def,
		Keyword: keyword, File: file, Line: lineNo,
	})
}

func (p *Parser) addEdge(from, to, kind, label, cond, file string, lineNo int) {
	if from == "" {
		from = "L:start"
	} else {
		from = "L:" + from
	}
	switch {
	case strings.HasPrefix(to, "expression:"):
		to = "L:expression"
	case strings.Contains(to, "."):
		// 子 label
		to = "L:" + to
	default:
		to = "L:" + to
	}
	key := from + ">" + to + ">" + label + ">" + cond
	if p.edgeSeen[key] {
		return
	}
	p.edgeSeen[key] = true
	p.edges = append(p.edges, model.FlowEdge{
		From: from, To: to, Kind: kind, Label: label, Cond: cond, File: file, Line: lineNo,
	})
}

func (p *Parser) finalize() {
	// 节点 = labels + 屏 + 外部未知目标
	for _, n := range p.labels {
		p.Model.Flow.Nodes = append(p.Model.Flow.Nodes, n)
	}
	for _, e := range p.edges {
		if _, ok := p.labels[e.To]; !ok && e.To != "" {
			// 外部（renpy 内部或 expression）
			if _, seen := p.labels[e.To]; !seen {
				p.labels[e.To] = model.FlowNode{ID: e.To, Label: strings.TrimPrefix(e.To, "L:"), Kind: "external"}
				p.Model.Flow.Nodes = append(p.Model.Flow.Nodes, p.labels[e.To])
			}
		}
		p.Model.Flow.Edges = append(p.Model.Flow.Edges, e)
	}
	sortNodes(p.Model.Flow.Nodes)

	// 台词计数 → 角色
	byTag := map[string]*model.Character{}
	for i := range p.Model.Characters {
		byTag[p.Model.Characters[i].Tag] = &p.Model.Characters[i]
	}
	if _, ok := byTag["narrator"]; !ok {
		n := model.Character{Tag: "narrator", Name: "旁白 (narrator)", Kind: "narrator"}
		p.Model.Characters = append(p.Model.Characters, n)
		byTag["narrator"] = &p.Model.Characters[len(p.Model.Characters)-1]
	}
	for _, d := range p.Model.Dialogues {
		if c, ok := byTag[d.Speaker]; ok {
			c.LineCnt++
		} else {
			// 未定义的说话人（如 extend / 直接名字）
			nc := model.Character{Tag: d.Speaker, Name: d.Speaker, Kind: "raw", LineCnt: 1}
			p.Model.Characters = append(p.Model.Characters, nc)
			byTag[d.Speaker] = &p.Model.Characters[len(p.Model.Characters)-1]
		}
	}
	sort.Slice(p.Model.Characters, func(i, j int) bool {
		return p.Model.Characters[i].LineCnt > p.Model.Characters[j].LineCnt
	})

	// stats
	st := model.Stats{
		DialogueCnt: len(p.Model.Dialogues),
		ChoiceCnt:   len(p.Model.Choices),
		FieldCnt:    len(p.Model.Fields),
		AssetKinds:  map[string]int{},
		AssetBytes:  map[string]int64{},
	}
	for _, n := range p.Model.Flow.Nodes {
		switch n.Kind {
		case "label", "external":
			st.Labels++
		case "screen":
			st.Screens++
		}
	}
	p.Model.Stats = st
}

func sortNodes(ns []model.FlowNode) {
	sort.Slice(ns, func(i, j int) bool { return ns[i].ID < ns[j].ID })
}

// ---- 小工具 ----

func guessType(expr string) string {
	e := strings.TrimSpace(expr)
	switch {
	case e == "True" || e == "False":
		return "bool"
	case strings.HasPrefix(e, "\"") || strings.HasPrefix(e, "'") || strings.HasPrefix(e, "_("):
		return "str"
	case strings.HasPrefix(e, "["):
		return "list"
	case strings.HasPrefix(e, "{"):
		return "dict"
	case strings.HasPrefix(e, "("):
		return "tuple"
	case regexp.MustCompile(`^-?\d+$`).MatchString(e):
		return "int"
	case regexp.MustCompile(`^-?\d*\.\d+$`).MatchString(e):
		return "float"
	case strings.HasPrefix(e, "Character"):
		return "Character"
	case strings.HasPrefix(e, "ImageReference") || strings.HasPrefix(e, "Image"):
		return "image"
	case e == "None" || e == "":
		return "none"
	default:
		return "pyobj"
	}
}

func firstStringArg(args string) string {
	// Character('Name', ...) / Character("Name", ...) / Character(None, ...)
	// 也支持 gettext 包装：Character(_("苏幼晴"), ...)
	if strings.HasPrefix(args, "None") {
		return ""
	}
	args = strings.TrimSpace(args)
	if strings.HasPrefix(args, "_(") {
		args = strings.TrimSpace(args[2:])
	}
	for i := 0; i < len(args); i++ {
		c := args[i]
		if c == '"' || c == '\'' {
			j := i + 1
			var sb strings.Builder
			for j < len(args) && args[j] != c {
				if args[j] == '\\' && j+1 < len(args) {
					sb.WriteByte(args[j+1])
					j += 2
					continue
				}
				sb.WriteByte(args[j])
				j++
			}
			return sb.String()
		}
		if c == ',' || c == '(' {
			return ""
		}
	}
	return ""
}

var reTagged = regexp.MustCompile(`\{[^}]*\}`)
var reInterp = regexp.MustCompile(`\[[^\]]*\]`)

// stripTags 去掉 {tag} 与 [] 插值，得到纯文本（保留插值占位符于 Text 字段）。
func stripTags(s string) string {
	s = reTagged.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return s
}

// unquote 解析 python 字符串字面量（含转义）。
func unquote(s string) string {
	s = strings.TrimSpace(s)
	// _(...)/u 前缀
	s = strings.TrimPrefix(s, "_(")
	s = strings.TrimSuffix(s, ")")
	for len(s) > 1 && (s[0] == 'u' || s[0] == 'r' || s[0] == 'b' || s[0] == 'f') &&
		(s[1] == '"' || s[1] == '\'') {
		s = s[1:]
	}
	if len(s) < 2 {
		return s
	}
	q := s[0]
	if (q != '"' && q != '\'') || s[len(s)-1] != q {
		return s
	}
	body := s[1 : len(s)-1]
	var sb strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '\\' && i+1 < len(body) {
			i++
			switch body[i] {
			case 'n':
				sb.WriteByte('\n')
			case 't':
				sb.WriteByte('\t')
			case 'r':
				sb.WriteByte('\r')
			case '"', '\'', '\\', '{', '[', '%':
				sb.WriteByte(body[i])
			case 'x':
				if i+2 < len(body) {
					if v, err := strconv.ParseUint(body[i+1:i+3], 16, 8); err == nil {
						sb.WriteByte(byte(v))
						i += 2
					}
				}
			case 'u':
				if i+4 < len(body) {
					if v, err := strconv.ParseUint(body[i+1:i+5], 16, 32); err == nil {
						sb.WriteRune(rune(v))
						i += 4
					}
				}
			case 'U':
				if i+8 < len(body) {
					if v, err := strconv.ParseUint(body[i+1:i+9], 16, 32); err == nil {
						sb.WriteRune(rune(v))
						i += 8
					}
				}
			default:
				sb.WriteByte(body[i])
			}
			continue
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

func extractFilePath(expr string) string {
	// image x = "file.webp" / im.Scale("f", ...) 等
	if i := strings.Index(expr, "\""); i >= 0 {
		j := strings.LastIndex(expr, "\"")
		if j > i {
			return expr[i+1 : j]
		}
	}
	if i := strings.Index(expr, "'"); i >= 0 {
		j := strings.LastIndex(expr, "'")
		if j > i {
			return expr[i+1 : j]
		}
	}
	return ""
}

func splitParams(s string) []string {
	out := []string{}
	depth := 0
	cur := strings.Builder{}
	for _, c := range s {
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, cur.String())
				cur.Reset()
				continue
			}
		}
		cur.WriteRune(c)
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, cur.String())
	}
	return out
}

func trimTo(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func setMeta(m map[string]any, k string, v any) map[string]any {
	if m == nil {
		m = map[string]any{}
	}
	m[k] = v
	return m
}
