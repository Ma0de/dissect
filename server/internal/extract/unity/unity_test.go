package unity

import (
	"strings"
	"testing"

	"dissect/internal/model"
)

const sampleDump = `// Image 0: mscorlib.dll - 0
// Image 1: Assembly-CSharp.dll - 1
namespace Game
{
	public class PlayerData : MonoBehaviour // TypeDefIndex: 100
	{
		// Fields
		public float moveSpeed; // 0x18
		public int hp; // 0x1C
		private string playerName; // 0x20
		public List<int> itemIds; // 0x28
		public static PlayerData Instance; // 0x0

		// Methods

		// RVA: 0x1234A0 Offset: 0x1232A0
		public void TakeDamage(int amount) { }

		// RVA: 0x1235B0 Offset: 0x1233B0
		public int get_Hp() { }
	}

	public enum GameState // TypeDefIndex: 101
	{
		// Fields
		public int value__; // 0x00
		public const GameState Title = 0;
		public const GameState Playing = 1;
	}
}
`

func TestParseDumpCS(t *testing.T) {
	m := &model.GameModel{Stats: model.Stats{}}
	parseDumpCS(sampleDump, m)
	var got []string
	for _, f := range m.Fields {
		got = append(got, f.Scope+"|"+f.Name+"|"+f.Type+"|"+f.Default)
	}
	want := []string{
		"ns:Game.PlayerData|moveSpeed|float|0x18",
		"ns:Game.PlayerData|hp|int|0x1C",
		"ns:Game.PlayerData|playerName|string|0x20",
		"ns:Game.PlayerData|itemIds|List<int>|0x28",
		"ns:Game.PlayerData|Instance|PlayerData|0x0",
	}
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want %d fields", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, got[i], want[i])
		}
	}
	if !strings.Contains(strings.Join(got, "\n"), "moveSpeed") {
		t.Fatal("missing moveSpeed")
	}
}
