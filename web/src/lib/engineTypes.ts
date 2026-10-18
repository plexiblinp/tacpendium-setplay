// エンジン(WASM の suggest)の結果の形。engine.Response と同じ(docs/engine.md)。
export type StepRole = "filler" | "target" | "parent";

export interface Step {
  code: string;
  role: StepRole;
  counted: boolean;
}

export type Mode = "meaty" | "gap" | "filler";

export interface Item {
  steps: Step[];
  s: number;
  n: number;
  landing: number;
  targetActive: number;
  mode: Mode;
  g: number;
  remain: number;
  /** レシピの同一性の鍵(engine の RecipeKey。S6)。投票と票の引き当てに使う。 */
  recipeKey: string;
}

export interface SuggestResponse {
  items: Item[];
  truncated: boolean;
  totalFound: number;
  reason?: string;
}

export interface EngineError {
  code: string;
  message: string;
  param?: string;
}
