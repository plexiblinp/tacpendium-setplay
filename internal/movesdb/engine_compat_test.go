package movesdb

import (
	"testing"

	setplayrepo "github.com/plexiblinp/tacpendium-setplay/internal/repository/setplay"
)

// エンジン側の読み口(internal/repository/setplay)が読める形の版は、moves DB の版と同じであること。
// moves DB の形を変えて SchemaVersion を上げたら、読み口も直すまでここが赤になる。
func TestSchemaVersionMatchesEngineReader(t *testing.T) {
	if setplayrepo.SchemaVersion != SchemaVersion {
		t.Fatalf("repository/setplay.SchemaVersion = %d, movesdb.SchemaVersion = %d", setplayrepo.SchemaVersion, SchemaVersion)
	}
}
