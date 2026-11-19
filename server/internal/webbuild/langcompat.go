package webbuild

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const langCompatBanner = `################################################################################
# Changing language
################################################################################
`

const langCompatHelpers = `def get_text_language():
    return renpy.game.preferences.text_language or renpy.game.preferences.language


def get_image_language():
    return renpy.game.preferences.image_language or renpy.game.preferences.language


def get_voice_language():
    return renpy.game.preferences.voice_language or renpy.game.preferences.language


IMAGE_EXTENSIONS = frozenset({".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".avif"})
AUDIO_EXTENSIONS = frozenset({".ogg", ".mp3", ".wav", ".flac", ".opus", ".aac", ".m4a", ".wma"})


def get_language_for_file(name=None, directory=None):
    if name:
        dot = name.rfind(".")
        if dot >= 0:
            ext = name[dot:].lower()
            if ext in IMAGE_EXTENSIONS:
                return get_image_language()
            elif ext in AUDIO_EXTENSIONS:
                return get_voice_language()

    if directory == "images":
        return get_image_language()
    elif directory == "audio":
        return get_voice_language()

    return get_text_language()
`

const langCompatChangeFuncs = `def change_text_language(language):
    old = renpy.game.preferences.text_language
    renpy.game.preferences.text_language = language

    effective = get_text_language()

    if effective is not None:
        renpy.exports.load_language(effective)

    if language == old:
        renpy.exports.restart_interaction()
        return

    tl = renpy.game.script.translator

    renpy.style.restore(style_backup)
    renpy.style.rebuild(False)

    gui_defaults = renpy.store.gui._defaults
    defaulted_variables = {name: getattr(renpy.store.gui, name) for name in gui_defaults}

    for i in renpy.config.translate_clean_stores:
        renpy.python.clean_store(i)

    for name, value in defaulted_variables.items():
        setattr(renpy.store.gui, name, value)

    renpy.store.gui._apply_rebuild()

    if renpy.config.new_translate_order:
        new_change_language(tl, effective, True)
    else:
        old_change_language(tl, effective, True)

    for i in renpy.config.translate_clean_stores:
        renpy.python.reset_store_changes(i)

    renpy.style.rebuild()
    renpy.display.tts.init()

    renpy.exports.block_rollback()
    renpy.exports.restart_interaction()


def change_image_language(language):
    old = renpy.game.preferences.image_language
    renpy.game.preferences.image_language = language

    if language != old:
        renpy.exports.free_memory()

    renpy.exports.restart_interaction()


def change_voice_language(language):
    renpy.game.preferences.voice_language = language
    renpy.exports.restart_interaction()
`

type langCompatEdit struct {
	old string
	new string
}

type langCompatPatch struct {
	file  string
	edits []langCompatEdit
}

var langCompatPatches = []langCompatPatch{
	{
		file: "renpy/preferences.py",
		edits: []langCompatEdit{
			{
				old: `# The language we use for translations.
Preference("language", None, (str, type(None)))
`,
				new: `# The language we use for translations.
Preference("language", None, (str, type(None)))
Preference("image_language", None, (str, type(None)))
Preference("text_language", None, (str, type(None)))
Preference("voice_language", None, (str, type(None)))
`,
			},
		},
	},
	{
		file: "renpy/loader.py",
		edits: []langCompatEdit{
			{
				old: `def get_prefixes(tl=True, directory=None):
    """
    Returns a list of prefixes to search for files.
    """

    rv = []

    if tl:
        language = renpy.game.preferences.language  # type: ignore
    else:
        language = None
`,
				new: `def get_prefixes(tl=True, directory=None, name=None):
    """
    Returns a list of prefixes to search for files.
    """

    rv = []

    if tl:
        language = renpy.translation.get_language_for_file(name=name, directory=directory)
    else:
        language = None
`,
			},
			{
				old: `    paths = [p + name for p in get_prefixes(directory=directory, tl=tl)]
`,
				new: `    paths = [p + name for p in get_prefixes(directory=directory, tl=tl, name=name)]
`,
			},
			{
				old: `    paths = [p + name for p in get_prefixes(tl=tl, directory=directory)]
`,
				new: `    paths = [p + name for p in get_prefixes(tl=tl, directory=directory, name=name)]
`,
			},
		},
	},
	{
		file: "renpy/execution.py",
		edits: []langCompatEdit{
			{
				old: `        self.translate_language = renpy.game.preferences.language  # type: ignore
`,
				new: `        self.translate_language = renpy.translation.get_text_language()  # type: ignore
`,
			},
		},
	},
	{
		file: "renpy/text/text.py",
		edits: []langCompatEdit{
			{
				old: `            self.language = renpy.game.preferences.language
`,
				new: `            self.language = renpy.translation.get_text_language()
`,
			},
			{
				old: `        if (self.language != renpy.game.preferences.language) and not self._uses_scope:
`,
				new: `        if (self.language != renpy.translation.get_text_language()) and not self._uses_scope:
`,
			},
		},
	},
	{
		file: "renpy/translation/__init__.py",
		edits: []langCompatEdit{
			{
				old: langCompatBanner,
				new: langCompatHelpers + "\n" + langCompatBanner,
			},
			{
				old: `    def lookup_translate(self, identifier, alternate=None) -> tuple[renpy.ast.Node, bool]:
        identifier = identifier.replace(".", "_")
        language = renpy.game.preferences.language
`,
				new: `    def lookup_translate(self, identifier, alternate=None) -> tuple[renpy.ast.Node, bool]:
        identifier = identifier.replace(".", "_")
        language = get_text_language()
`,
			},
			{
				old: `    if language is Default:
        language = renpy.game.preferences.language
`,
				new: `    if language is Default:
        language = get_text_language()
`,
			},
			{
				old: `def write_updated_strings():
    stl = renpy.game.script.translator.strings[renpy.game.preferences.language]
    stl.write_updated_strings(renpy.game.preferences.language)
`,
				new: `def write_updated_strings():
    language = get_text_language()
    stl = renpy.game.script.translator.strings[language]
    stl.write_updated_strings(language)
`,
			},
			{
				old: `    if language != old_language:
        renpy.exports.block_rollback()

    old_language = language


def check_language():
`,
				new: `    if language != old_language:
        renpy.exports.block_rollback()

    old_language = language


` + langCompatChangeFuncs + `

def check_language():
`,
			},
			{
				old: `    ctx = renpy.game.contexts[-1]
    preferences = renpy.game.preferences

    # Deal with a changed language.
    if ctx.translate_language != preferences.language:
        clean_data()

        ctx.translate_language = preferences.language
`,
				new: `    ctx = renpy.game.contexts[-1]

    # Deal with a changed language.
    text_language = get_text_language()

    if ctx.translate_language != text_language:
        clean_data()

        ctx.translate_language = text_language
`,
			},
		},
	},
	{
		file: "renpy/common/00defaults.rpy",
		edits: []langCompatEdit{
			{
				old: `            if config.default_language is not None:
                _preferences.language = config.default_language
`,
				new: `            if config.default_language is not None:
                _preferences.language = config.default_language
                _preferences.text_language = config.default_language
                _preferences.image_language = config.default_language
                _preferences.voice_language = config.default_language
`,
			},
		},
	},
}

var langCompatPattern = regexp.MustCompile(`\b(?:image_language|text_language|voice_language)\b|change_(?:image|text|voice)_language`)

func needsLangCompat(outDir string) bool {
	found := false
	dir := filepath.Join(outDir, "scripts")
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(p), ".rpy") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if langCompatPattern.Match(b) {
			found = true
		}
		return nil
	})
	return found
}

func sdkHasLangCompat(sdkDir string) bool {
	b, err := os.ReadFile(filepath.Join(sdkDir, "renpy", "preferences.py"))
	if err != nil {
		return false
	}
	return bytes.Contains(b, []byte(`Preference("image_language"`))
}

func findLangCompatPatch(rel string) *langCompatPatch {
	for i := range langCompatPatches {
		if langCompatPatches[i].file == rel {
			return &langCompatPatches[i]
		}
	}
	return nil
}

func (p *langCompatPatch) apply(data []byte) ([]byte, error) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, e := range p.edits {
		if !strings.Contains(s, e.old) {
			return nil, fmt.Errorf("运行库源码与预期不符：%s", strings.SplitN(e.old, "\n", 2)[0])
		}
		s = strings.Replace(s, e.old, e.new, 1)
	}
	return []byte(s), nil
}
