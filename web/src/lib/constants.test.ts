import { describe, expect, it } from "vitest";

import { FEEDBACK_FORM_URL, FEEDBACK_URL, ISSUES_URL, feedbackUrl } from "./constants";

describe("意見を送る先(S7)", () => {
  it("フォームがあればフォーム、空なら Issues", () => {
    expect(feedbackUrl("https://docs.google.com/forms/d/e/x/viewform")).toBe("https://docs.google.com/forms/d/e/x/viewform");
    expect(feedbackUrl("")).toBe(ISSUES_URL);
  });
  it("フォームの URL は Google フォームの https で、追跡用の引数を付けない", () => {
    expect(FEEDBACK_FORM_URL).toMatch(/^https:\/\/docs\.google\.com\/forms\/d\/e\/[A-Za-z0-9_-]+\/viewform$/);
    expect(FEEDBACK_URL).toBe(FEEDBACK_FORM_URL);
  });
});
