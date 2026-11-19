// Package renpy: export.go —— 导出 dialogues/choices/fields/flow 等交付物。
package renpy

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dissect/internal/model"
)

func writeExports(m *model.GameModel, dir string) error {
	// dialogues.json
	if b, err := json.MarshalIndent(m.Dialogues, "", "  "); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "dialogues.json"), b, 0o644); err != nil {
			return err
		}
	}
	// dialogues.csv（按角色分组的传统字幕格式）
	if err := writeCSV(filepath.Join(dir, "dialogues.csv"),
		[]string{"id", "file", "line", "speaker", "label", "text", "condition", "audio"},
		len(m.Dialogues), func(i int) []string {
			d := m.Dialogues[i]
			return []string{itoa(d.ID), d.File, itoa(d.Line), d.Speaker, d.Label, d.Text, d.Cond, d.Audio}
		}); err != nil {
		return err
	}
	// dialogues_by_character.csv（用户重点需求：不同角色剧情文案）
	speakerOrder := make([]string, 0)
	for _, c := range m.Characters {
		speakerOrder = append(speakerOrder, c.Tag)
	}
	if err := os.WriteFile(filepath.Join(dir, "dialogues_by_character.txt"),
		[]byte(byCharacter(m)), 0o644); err != nil {
		return err
	}

	// choices.csv
	if err := writeCSV(filepath.Join(dir, "choices.csv"),
		[]string{"id", "file", "line", "label", "text", "condition", "target", "target_kind"},
		len(m.Choices), func(i int) []string {
			c := m.Choices[i]
			return []string{itoa(c.ID), c.File, itoa(c.Line), c.Label, c.Text, c.Cond, c.Target, c.TargetKind}
		}); err != nil {
		return err
	}

	// fields.json + fields.csv
	if b, err := json.MarshalIndent(m.Fields, "", "  "); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "fields.json"), b, 0o644); err != nil {
			return err
		}
	}
	if err := writeCSV(filepath.Join(dir, "fields.csv"),
		[]string{"scope", "name", "type", "keyword", "default", "file", "line"},
		len(m.Fields), func(i int) []string {
			f := m.Fields[i]
			return []string{f.Scope, f.Name, f.Type, f.Keyword, f.Default, f.File, itoa(f.Line)}
		}); err != nil {
		return err
	}

	// flow.json + flow.mmd
	if b, err := json.MarshalIndent(m.Flow, "", "  "); err == nil {
		if err := os.WriteFile(filepath.Join(dir, "flow.json"), b, 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "flow.mmd"), []byte(Mermaid(m)), 0o644); err != nil {
		return err
	}
	return nil
}

func byCharacter(m *model.GameModel) string {
	var sb strings.Builder
	for _, c := range m.Characters {
		if c.LineCnt == 0 {
			continue
		}
		sb.WriteString("==================================================\n")
		sb.WriteString(fmt.Sprintf("【%s】(%s) — %d 句\n", c.Name, c.Tag, c.LineCnt))
		sb.WriteString("==================================================\n")
		for _, d := range m.Dialogues {
			if d.Speaker == c.Tag {
				loc := fmt.Sprintf("(%s:%d)", d.File, d.Line)
				sb.WriteString(fmt.Sprintf("%s %s\n", loc, d.Text))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func writeCSV(path string, header []string, n int, row func(int) []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		if err := w.Write(row(i)); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }

// Mermaid 生成流程图的 Mermaid 源码。
func Mermaid(m *model.GameModel) string {
	var sb strings.Builder
	sb.WriteString("flowchart TD\n")
	names := map[string]string{}
	for _, n := range m.Flow.Nodes {
		names[n.ID] = n.Label
		if n.Kind == "screen" {
			sb.WriteString(fmt.Sprintf("  %s[[%s (screen)]]\n", safeID(n.ID), n.Label))
		} else {
			sb.WriteString(fmt.Sprintf("  %s[%s]\n", safeID(n.ID), n.Label))
		}
	}
	for _, e := range m.Flow.Edges {
		lbl := ""
		switch {
		case e.Label != "":
			lbl = e.Label
		case e.Cond != "":
			lbl = "if " + e.Cond
		}
		mark := "-->"
		if e.Kind == "call" || e.Kind == "call-screen" {
			mark = "-.->"
		}
		if lbl != "" {
			sb.WriteString(fmt.Sprintf("  %s %s|%s| %s\n",
				safeID(e.From), mark, escapeMMD(lbl), safeID(e.To)))
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s %s\n", safeID(e.From), mark, safeID(e.To)))
		}
	}
	return sb.String()
}

func safeID(id string) string {
	r := strings.NewReplacer(":", "_", ".", "_", "-", "_", " ", "_", "[", "_", "]", "_",
		"(", "_", ")", "_", "/", "_", "\\", "_", "\"", "_", "'", "_")
	return "n_" + r.Replace(id)
}

func escapeMMD(s string) string {
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}

// unused: 防止 import 误删
var _ = sort.Strings
var _ = filepath.Join
