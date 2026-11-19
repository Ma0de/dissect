// Package renpy: header.go —— 生成「类 IL2CPP.h」C++ 头文件。
// 把 Ren'Py 游戏的数据模型（store 变量 / python 类字段 / 角色定义 / screen 参数）
// 映射为带类型与顺序注释的 C++ 结构体，供第三方开发者做存档编辑器、字段扫描类修改工具。
package renpy

import (
	"fmt"
	"sort"
	"strings"

	"dissect/internal/model"
)

// cppType 把 Ren'Py/Python 值类型映射为 C++ 表达。
func cppType(t string) string {
	switch t {
	case "int":
		return "py_int"
	case "float":
		return "py_float"
	case "bool":
		return "py_bool"
	case "str":
		return "py_str"
	case "list":
		return "py_list"
	case "dict":
		return "py_dict"
	case "tuple":
		return "py_tuple"
	case "Character":
		return "py_obj /*Character*/"
	case "image":
		return "py_obj /*image*/"
	case "none":
		return "py_obj /*None*/"
	default:
		return "py_obj"
	}
}

func cppIdent(s string) string {
	r := strings.NewReplacer(".", "_", "-", "_", " ", "_", "[", "_", "]", "_",
		"(", "_", ")", "_", "/", "_", "'", "_", "\"", "_", "{", "_", "}", "_")
	id := r.Replace(s)
	if id == "" {
		return "_"
	}
	if !(id[0] >= 'a' && id[0] <= 'z' || id[0] >= 'A' && id[0] <= 'Z' || id[0] == '_') {
		id = "_" + id
	}
	return id
}

func cppStructName(scope string) string {
	switch {
	case scope == "store":
		return "renpy_store"
	case strings.HasPrefix(scope, "class:"):
		return "cls_" + cppIdent(strings.TrimPrefix(scope, "class:"))
	case strings.HasPrefix(scope, "screen:"):
		return "screen_" + cppIdent(strings.TrimPrefix(scope, "screen:"))
	case strings.HasPrefix(scope, "character:"):
		return "char_" + cppIdent(strings.TrimPrefix(scope, "character:"))
	default:
		return "unknown_" + cppIdent(scope)
	}
}

// GenerateHeader 生成 Model.h 内容。
func GenerateHeader(m *model.GameModel) string {
	gameID := cppIdent(m.GameID)
	if gameID == "_" {
		gameID = "game"
	}

	// 按 scope 分组，保持声明顺序
	groups := map[string][]model.Field{}
	order := []string{}
	for _, f := range m.Fields {
		if _, ok := groups[f.Scope]; !ok {
			order = append(order, f.Scope)
		}
		groups[f.Scope] = append(groups[f.Scope], f)
	}
	sort.SliceStable(order, func(i, j int) bool {
		rank := func(s string) int {
			switch {
			case s == "store":
				return 0
			case strings.HasPrefix(s, "class:"):
				return 1
			case strings.HasPrefix(s, "screen:"):
				return 2
			case strings.HasPrefix(s, "character:"):
				return 3
			}
			return 4
		}
		if rank(order[i]) != rank(order[j]) {
			return rank(order[i]) < rank(order[j])
		}
		return order[i] < order[j]
	})

	var sb strings.Builder
	w := func(format string, args ...any) { sb.WriteString(fmt.Sprintf(format, args...)) }

	w(`// ============================================================================
// Model.h — dissect 生成
// 游戏: %s   引擎: Ren'Py %s
//
// 说明: Ren'Py 存档为 Python pickle，字段无固定内存偏移；下方 `+"`slot`"+` 注释
//       表示该字段在 store 声明顺序中的序号，可用于 pickle 工具（如
//       pickletools / save 编辑脚本）定位。py_* 类型定义见文件底部。
// ============================================================================
#pragma once
#include <cstdint>

namespace dissect_%s {

typedef int64_t   py_int;      // python int（存档中可任意大，此处按常用范围）
typedef double    py_float;    // python float
typedef uint8_t   py_bool;     // True/False
typedef const char* py_str;    // python str
typedef void*     py_obj;      // 任意 python 对象
typedef py_obj py_list;        // list
typedef py_obj py_dict;        // dict
typedef py_obj py_tuple;       // tuple

`, m.GameName, m.EngineVersion, gameID)

	// 前置声明 + 结构体
	for i, scope := range order {
		fields := groups[scope]
		// 去重（同名取首个）
		seen := map[string]bool{}
		uniq := make([]model.Field, 0, len(fields))
		for _, f := range fields {
			if seen[f.Name] {
				continue
			}
			seen[f.Name] = true
			uniq = append(uniq, f)
		}
		w("// %s (%d 个字段)\n", scope, len(uniq))
		w("struct %s {\n", cppStructName(scope))
		for slot, f := range uniq {
			name := cppIdent(f.Name)
			w("    %-9s %-28s // slot=%d  %s:%d  (%s) %s\n",
				cppType(f.Type), name+";", slot, f.File, f.Line, f.Keyword, f.Default)
		}
		if len(uniq) == 0 {
			w("    // (无字段)\n")
		}
		w("};\n\n")
		_ = i
	}

	// 角色表
	if len(m.Characters) > 0 {
		w("// 角色台词统计（供文案工具 / 修改器对照说话人）\n")
		w("// tag | name | lines\n")
		for _, c := range m.Characters {
			w("// %-20s %-24s %d\n", c.Tag, c.Name, c.LineCnt)
		}
		w("\n")
	}

	w(`} // namespace dissect_%s
`, gameID)
	return sb.String()
}
