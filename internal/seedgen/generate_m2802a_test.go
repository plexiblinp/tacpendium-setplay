package seedgen

import (
	"strings"
	"testing"
)

// M28-02a §2.2-4: FR702 のマーカー(last_changed_game_version)が seed 再生成で消えないこと。
//
// ★★本サブで 2 番目に重要な罠がここにある。
//   seedgen が SQL へ出力しない列は golden が守らず、DB だけが進む。
//   先例が 3 列ある(startup_basis / chain_cancel_total / fastest_unreachable
//   = followup `csv-db-frame-cost-columns-drift`。実測 2720 セルが空だった)。
//   ★moves の seed には DELETE → INSERT で id が再発番される波が実在するため、
//     マーカーを DML マイグレだけで持つと、その波が黙って消す。
//   ★★どちらもエラーにならない。⇒ ここで固定する以外に気づく経路が無い。

// genGameVersion は 1 キャラ分の CSV 本文から -mode game-version の生成を回す小道具。
func genGameVersion(t *testing.T, code, body string) (*Result, error) {
	t.Helper()
	rows, err := Read(strings.NewReader(testHeader+body), 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return GenerateGameVersion([]string{code}, map[string][]MoveRow{code: rows},
		GameVersionHeader("zz_test_game_version", "テスト"))
}

// TestGenerate_GameVersionEmittedWhenPresent は、CSV に版数が入っていれば
// -mode game-version の生成 SQL が UPDATE を持つことを固定する。
func TestGenerate_GameVersionEmittedWhenPresent(t *testing.T) {
	body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,,,,2026.08.03.01,\n" +
		"terry,crouching_light_punch,normal,しゃがみ弱P,4,3,7,13,4,-1,300,false,false,false,,,,d plus p_l,,,,,,2026.08.03.01,\n" +
		"terry,power_wave_light,special,弱パワーウェイブ,14,36,0,49,-3,-9,600,false,true,false,,,,d dr r plus p_l,,,,,,2026.09.01.00,\n" +
		"terry,buster_wolf,super_art,バスターウルフ,9,4,44,56,,-22,2000,false,false,false,,,,d dr r d dr r plus k,,,,,,,\n"
	res, err := genGameVersion(t, "terry", body)
	if err != nil {
		t.Fatalf("GenerateGameVersion: %v", err)
	}
	up := res.UpSQL

	// 同じ版で変わった技は 1 本の UPDATE にまとまる。
	if !strings.Contains(up, "UPDATE moves SET last_changed_game_version = '2026.08.03.01'") {
		t.Errorf("2026.08.03.01 の UPDATE が出ていない:\n%s", up)
	}
	if !strings.Contains(up, "UPDATE moves SET last_changed_game_version = '2026.09.01.00'") {
		t.Errorf("2026.09.01.00 の UPDATE が出ていない:\n%s", up)
	}
	if got := strings.Count(up, "UPDATE moves SET last_changed_game_version"); got != 2 {
		t.Errorf("UPDATE の本数 = %d, want 2(版数ごとに 1 本):\n%s", got, up)
	}
	// 版数ごとに対象 code がまとまっている。
	if !strings.Contains(up, "'standing_light_punch', 'crouching_light_punch'") {
		t.Errorf("同一版の 2 技が 1 本の UPDATE にまとまっていない:\n%s", up)
	}
	// ★空欄の技は対象に入らない(「まだ変わっていない」を「変わった」にしない)。
	//   ★UPDATE 文だけを切り出して見る —— 後続の alias INSERT にも技 code は現れるため、
	//     生成物全体を対象にすると、この検査は常に赤くなる。
	for _, stmt := range gameVersionUpdateStatements(up) {
		if strings.Contains(stmt, "buster_wolf") {
			t.Errorf("版数が空欄の技が UPDATE の対象に入っている:\n%s", stmt)
		}
	}
	// ★キャラで絞られている(他キャラの同名 code を巻き込まない)。
	if !strings.Contains(up, "WHERE character_id IN (SELECT c.id FROM characters c WHERE c.code = 'terry'") {
		t.Errorf("UPDATE がキャラで絞られていない:\n%s", up)
	}
	// ★出力は決定論であること(版数の昇順)。map の反復順に依存すると golden が揺れる。
	i1 := strings.Index(up, "'2026.08.03.01'")
	i2 := strings.Index(up, "'2026.09.01.00'")
	if i1 < 0 || i2 < 0 || i1 > i2 {
		t.Errorf("UPDATE の並びが版数昇順でない(決定論でない):\n%s", up)
	}
}

// TestGenerate_GameVersionNotEmittedWhenAllEmpty は、値が 1 つも無ければ
// 1 バイトも出力しないことを固定する。
//
// ★★これが golden テスト 17 本を byte-identical のまま通す条件である。
//
//	現状の character_data/*.csv は全行が空欄であるため、生成物は列追加前と完全に一致する。
//	⇒ 適用済みマイグレを再生成せずに済む(assertGolden が禁じている)。
func TestGenerate_GameVersionNotEmittedWhenAllEmpty(t *testing.T) {
	body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,standalone,9,true,,\n"
	res, err := gen(t, "terry", body)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, s := range []string{res.UpSQL, res.DownSQL} {
		if strings.Contains(s, "last_changed_game_version") {
			t.Errorf("値が 1 つも無いのに生成 SQL へ列名が出ている:\n%s", s)
		}
	}
	// 技自体は(マーカーなしで)seed される。
	if !strings.Contains(res.UpSQL, "'standing_light_punch'") {
		t.Errorf("技が seed されていない:\n%s", res.UpSQL)
	}
}

// TestRead_InvalidGameVersionFails は seedgen 側の入口(値域検証)を固定する。
//
// ★★指示書 §2.2-3-b の「入口を全部塞ぐ」のうち、CSV から入る経路がこれである。
//
//	ゼロ埋めの崩れた版数が 1 つ入るだけで辞書順が狂い、★エラーにはならない。
//	⇒ 生成そのものを fail させる(startup_basis と同じ流儀)。
func TestRead_InvalidGameVersionFails(t *testing.T) {
	bad := []string{
		"2026.8.3.1",    // ★指示書が名指しする形。ゼロ埋めなし
		"2026.08.03.1",  // アプリ版だけ 1 桁
		"2026.08.03",    // アプリ版なし
		"2026.13.01.01", // 月が範囲外
		"v2026.08.03.01",
	}
	for _, v := range bad {
		body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,,,," + v + ",\n"
		if _, err := gen(t, "terry", body); err == nil {
			t.Errorf("崩れた版数 %q が seedgen を通った(値域検証が効いていない)", v)
		}
	}
	// 正しい形と空欄は通る。
	for _, v := range []string{"2026.08.03.01", "2026.12.31.99", ""} {
		body := "terry,standing_light_punch,normal,立ち弱P,4,3,7,13,4,-1,300,false,false,false,,,,p_l,,,,,," + v + ",\n"
		if _, err := gen(t, "terry", body); err != nil {
			t.Errorf("正しい版数 %q が弾かれた: %v", v, err)
		}
	}
}

// gameVersionUpdateStatements は生成 SQL から
// `UPDATE moves SET last_changed_game_version ...;` の文だけを切り出す。
func gameVersionUpdateStatements(sql string) []string {
	const head = "UPDATE moves SET last_changed_game_version"
	var out []string
	rest := sql
	for {
		i := strings.Index(rest, head)
		if i < 0 {
			return out
		}
		rest = rest[i:]
		end := strings.Index(rest, ";")
		if end < 0 {
			return append(out, rest)
		}
		out = append(out, rest[:end+1])
		rest = rest[end+1:]
	}
}
