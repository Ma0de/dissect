// Package model 定义 dissect 的通用「游戏模型」——
// 不同引擎 (Ren'Py / Unity IL2CPP ...) 解包后统一归一化到此结构，
// 前端 UI 与导出 (CSV / JSON / Mermaid / C++ 头文件) 全部消费该模型。
package model

// GameModel 是解包产出的根对象，序列化为 outdir/model.json。
type GameModel struct {
	GameName      string         `json:"gameName"`
	GameID        string         `json:"gameId"` // 引擎内代号，如 TinyShadowsInterwovenHearts
	Engine        string         `json:"engine"` // renpy / unity / unknown
	EngineVersion string         `json:"engineVersion"`
	SourceDir     string         `json:"sourceDir"` // 分析的输入路径
	Stats         Stats          `json:"stats"`
	Characters    []Character    `json:"characters"`
	Fields        []Field        `json:"fields"`
	Dialogues     []Dialogue     `json:"dialogues"`
	Choices       []Choice       `json:"choices"`
	Flow          FlowGraph      `json:"flow"`
	Assets        []Asset        `json:"assets"`
	ScriptFiles   []ScriptFile   `json:"scriptFiles"`
	Headers       []HeaderFile   `json:"headers"`              // 生成/收集到的 C++ 头文件 (类 IL2CPP.h)
	Notes         []string       `json:"notes"`                // 分析过程中的提示与警告
	EngineMeta    map[string]any `json:"engineMeta,omitempty"` // 引擎特有信息
}

type Stats struct {
	Labels      int              `json:"labels"`
	Screens     int              `json:"screens"`
	DialogueCnt int              `json:"dialogueCount"`
	ChoiceCnt   int              `json:"choiceCount"`
	FieldCnt    int              `json:"fieldCount"`
	VoiceMapped int              `json:"voiceMapped"`
	AssetKinds  map[string]int   `json:"assetKinds"`
	AssetBytes  map[string]int64 `json:"assetBytes"`
}

// Character 游戏中的说话角色（Ren'Py Character() 定义）。
type Character struct {
	Tag     string   `json:"tag"`  // 变量名 / 脚本内引用名
	Name    string   `json:"name"` // 显示名（含动态占位符如 [player_name]）
	Color   string   `json:"color,omitempty"`
	Kind    string   `json:"kind,omitempty"` // normal / narrator / dynamic
	LineCnt int      `json:"lineCount"`
	Sources []string `json:"sources,omitempty"` // 定义位置 file:line
}

// Field 一切可被修改软件（存档编辑器等）瞄准的“字段”：
// define/default 的 store 变量、python 类属性、screen 参数、Character 定义参数。
type Field struct {
	Name    string `json:"name"`    // store 内完整名，如 affection 或 player.name
	Scope   string `json:"scope"`   // store / class:ClassName / screen:name / character:tag
	Type    string `json:"type"`    // 推断类型 int/float/bool/str/list/dict/pyobj/...
	Default string `json:"default"` // 字面默认值表达式（原样保留）
	Keyword string `json:"keyword"` // define / default / self / screen-param
	File    string `json:"file"`
	Line    int    `json:"line"`
	ClassOf string `json:"classOf,omitempty"` // Scope=class 时所属 python 类名
}

type Dialogue struct {
	ID       int    `json:"id"` // 全局顺序编号（≈游戏流程中的叙述顺序）
	File     string `json:"file"`
	Line     int    `json:"line"`
	Speaker  string `json:"speaker"` // 角色变量名，"narrator" 表示旁白
	Text     string `json:"text"`    // 原始文本（保留 {tag} 与 [变量]）
	Plain    string `json:"plain"`   // 去掉标记后的纯文本
	Cond     string `json:"cond,omitempty"`
	Label    string `json:"label"` // 所属 label
	WithExpr string `json:"withExpr,omitempty"`
	VoiceID  string `json:"voiceId,omitempty"` // Ren'Py auto_voice 翻译 id
	Audio    string `json:"audio,omitempty"`   // 匹配到的语音资产（相对 assets/）
}

type Choice struct {
	ID         int    `json:"id"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Label      string `json:"label"` // 所属 label（menu 所在处）
	Text       string `json:"text"`
	Cond       string `json:"cond,omitempty"`
	Target     string `json:"target,omitempty"`     // 选项块内首个 jump/call 目标
	TargetKind string `json:"targetKind,omitempty"` // jump / call / inline
}

type FlowGraph struct {
	Nodes []FlowNode `json:"nodes"`
	Edges []FlowEdge `json:"edges"`
}

type FlowNode struct {
	ID    string `json:"id"`
	Label string `json:"label"` // 显示名
	Kind  string `json:"kind"`  // label / screen / external
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
}

type FlowEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"`            // jump / call / choice / fallthrough
	Label string `json:"label,omitempty"` // choice 文本或条件
	Cond  string `json:"cond,omitempty"`
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
}

type Asset struct {
	Path string `json:"path"` // 相对导出 assets/ 目录路径
	Kind string `json:"kind"` // image / audio / font / video / other / archive / script
	Size int64  `json:"size"`
	// 引用该资源的脚本位置（image/show/scene 语句等）
	UsedAt []string `json:"usedAt,omitempty"`
	// Ren'Py image 别名
	ImageName string `json:"imageName,omitempty"`
}

type ScriptFile struct {
	Path       string `json:"path"`
	Lines      int    `json:"lines"`
	Decompiled bool   `json:"decompiled"` // 是否成功由 .rpyc 反编译而来
}

type HeaderFile struct {
	Name    string `json:"name"` // Model.h / il2cpp.h / dump.cs ...
	Engine  string `json:"engine"`
	Note    string `json:"note"`
	Content string `json:"-"`
	Path    string `json:"path"` // 导出文件路径
}

// Normalize 把所有 nil 的切片/Map 归一化为空集合，
// 保证 model.json 中永远是 [] / {} 而不是 null（前端对 null.length 会崩溃）。
func (m *GameModel) Normalize() {
	if m.Stats.AssetKinds == nil {
		m.Stats.AssetKinds = map[string]int{}
	}
	if m.Stats.AssetBytes == nil {
		m.Stats.AssetBytes = map[string]int64{}
	}
	if m.Characters == nil {
		m.Characters = []Character{}
	}
	if m.Fields == nil {
		m.Fields = []Field{}
	}
	if m.Dialogues == nil {
		m.Dialogues = []Dialogue{}
	}
	if m.Choices == nil {
		m.Choices = []Choice{}
	}
	if m.Flow.Nodes == nil {
		m.Flow.Nodes = []FlowNode{}
	}
	if m.Flow.Edges == nil {
		m.Flow.Edges = []FlowEdge{}
	}
	if m.Assets == nil {
		m.Assets = []Asset{}
	}
	if m.ScriptFiles == nil {
		m.ScriptFiles = []ScriptFile{}
	}
	if m.Headers == nil {
		m.Headers = []HeaderFile{}
	}
	if m.Notes == nil {
		m.Notes = []string{}
	}
}

// 统计角色台词数等需要遍历 Dialogues 后回填，由 Extractor 实现。
