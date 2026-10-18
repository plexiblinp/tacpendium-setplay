//go:build js && wasm

// Command wasm はセットプレイ提案のエンジン(engine パッケージ)を WASM としてブラウザ ・Node へ出す。
//
// ビルド: GOOS=js GOARCH=wasm go build -o setplay.wasm ./cmd/wasm
// 読み込み側は Go 付属の wasm_exec.js(`go env GOROOT`/lib/wasm/)で起動する。起動すると
// globalThis.tacpendiumSetplay に次の関数を置く(どれも JSON 文字列を返し、例外を投げない):
//
//	load(movesJson: string) → {"character": "ryu"} | {"error": {...}}
//	suggest(character: string, ka: number, query: string, votes?: string) → {"result": {...}} | {"error": {...}}
//
// query は本家の API と同じ名前のクエリ文字列(例 "mode=gap&g_min=2")。結果の形は engine.Response。
// votes は票(S6)の JSON({"<recipeKey>": good − bad})。省略 ・空文字は票なし(並びは本家と同じ)。
// 計算の中断は Web Worker ごと止める(HANDOVER §2.1。本コマンドは取り消しを受けない)。
package main

import (
	"syscall/js"

	"github.com/plexiblinp/tacpendium-setplay/engine"
)

// globalName は JS 側に置くオブジェクトの名前。
const globalName = "tacpendiumSetplay"

func main() {
	e := engine.New()
	api := js.Global().Get("Object").New()
	api.Set("load", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return string(engine.InvalidArgumentsJSON("load(movesJson: string)"))
		}
		return string(e.LoadJSON([]byte(args[0].String())))
	}))
	api.Set("suggest", js.FuncOf(func(this js.Value, args []js.Value) any {
		const usage = "suggest(character: string, ka: number, query: string, votes?: string)"
		if len(args) < 3 || len(args) > 4 || args[0].Type() != js.TypeString || args[1].Type() != js.TypeNumber || args[2].Type() != js.TypeString {
			return string(engine.InvalidArgumentsJSON(usage))
		}
		votes := ""
		if len(args) == 4 {
			if args[3].Type() != js.TypeString {
				return string(engine.InvalidArgumentsJSON(usage))
			}
			votes = args[3].String()
		}
		ka, ok := engine.KnockdownFromFloat(args[1].Float())
		if !ok {
			return string(engine.InvalidArgumentsJSON("ka は整数(NaN ・Infinity ・小数は不可)"))
		}
		return string(e.SuggestWithVotesJSON(args[0].String(), ka, args[2].String(), votes))
	}))
	js.Global().Set(globalName, api)
	select {}
}
