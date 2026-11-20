package renpy

import "testing"

func TestParseCRLFDialogue(t *testing.T) {
	src := "label start:\r\n" +
		"    \"旁白一句。\"\r\n" +
		"    e \"角色一句。\"\r\n" +
		"    menu:\r\n" +
		"        \"继续前进\":\r\n" +
		"            jump next_scene\r\n" +
		"    return\r\n" +
		"\r\n" +
		"label next_scene:\r\n" +
		"    e \"到了。\"\r\n" +
		"    return\r\n"
	p := NewParser()
	p.ParseFiles(map[string]string{"test.rpy": src})
	if len(p.Model.Dialogues) != 3 {
		t.Fatalf("dialogues = %d, want 3", len(p.Model.Dialogues))
	}
	if len(p.Model.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(p.Model.Choices))
	}
	if len(p.Model.Flow.Nodes) < 2 {
		t.Fatalf("flow nodes = %d, want >= 2", len(p.Model.Flow.Nodes))
	}
}

func TestParseLFDialogue(t *testing.T) {
	src := "label start:\n" +
		"    e \"角色一句。\"\n" +
		"    return\n"
	p := NewParser()
	p.ParseFiles(map[string]string{"test.rpy": src})
	if len(p.Model.Dialogues) != 1 {
		t.Fatalf("dialogues = %d, want 1", len(p.Model.Dialogues))
	}
}

func TestUnquoteExpressions(t *testing.T) {
	cases := []struct{ in, want string }{
		{`_("永恒与星辰与日常") + (" Demo" if _obe_demo_build else "")`, "永恒与星辰与日常"},
		{`"Hello"`, "Hello"},
		{`'single'`, "single"},
		{`u"中文\n"`, "中文\n"},
		{`"A\u4e2dB"`, "A中B"},
		{`_("translated")`, "translated"},
		{`some_call("x")`, `some_call("x")`},
		{`"a" + "b"`, "a"},
		{`_("x") + y`, "x"},
		{`_("name: %s" % who)`, "name: %s"},
	}
	for _, c := range cases {
		if got := unquote(c.in); got != c.want {
			t.Fatalf("unquote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseConfigNameExpression(t *testing.T) {
	src := "define config.name = _(\"永恒与星辰与日常\") + (\" Demo\" if _obe_demo_build else \"\")\n"
	p := NewParser()
	p.ParseFiles(map[string]string{"options.rpy": src})
	if p.Model.GameName != "永恒与星辰与日常" {
		t.Fatalf("game name = %q", p.Model.GameName)
	}
}
