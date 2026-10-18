-- 票(D17)と日替わりソルト(D14)の D1 のスキーマ(S6)。
-- 適用: npx wrangler d1 execute <DB 名> --remote --file=edge/schema.sql(開発者が行う。docs/telemetry.md)
-- 何度流しても同じになる(IF NOT EXISTS)。

-- 日替わりソルト。利用者のハッシュ hash(IP + UA + ソルト) に使い、翌日に消す(生の IP は保存しない)。
CREATE TABLE IF NOT EXISTS salts (
  day  TEXT PRIMARY KEY, -- UTC の日付 YYYY-MM-DD
  salt TEXT NOT NULL     -- 乱数(16 進 64 桁)
) WITHOUT ROWID;

-- レシピごとの票の数(キャラ・KA ごと。開発者決定 2026-10-12)。配信はこの表を主キーの前方一致で読む。
CREATE TABLE IF NOT EXISTS votes (
  char   TEXT    NOT NULL,
  ka     INTEGER NOT NULL,
  recipe TEXT    NOT NULL, -- engine の RecipeKey
  good   INTEGER NOT NULL DEFAULT 0,
  bad    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (char, ka, recipe)
) WITHOUT ROWID;

-- 今日の投票(重複防止: 利用者のハッシュ × レシピ × 日で 1 票)。前日以前の行は消す(ハッシュはソルトと一緒に捨てる)。
CREATE TABLE IF NOT EXISTS ballots (
  day    TEXT    NOT NULL,
  voter  TEXT    NOT NULL,
  char   TEXT    NOT NULL,
  ka     INTEGER NOT NULL,
  recipe TEXT    NOT NULL,
  value  INTEGER NOT NULL, -- 1 = グッド / -1 = バッド
  PRIMARY KEY (day, voter, char, ka, recipe)
) WITHOUT ROWID;
