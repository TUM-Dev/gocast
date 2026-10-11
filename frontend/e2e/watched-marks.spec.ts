import { expect, test } from "@playwright/test";

import { apiAs, bearerToken, login } from "./helpers";
import { courseUrl, markable, users } from "./seed";

/**
 * The course page's lecture menu marks a lecture watched and, when it is, unmarks it.
 * Unmarking used to do nothing: the server only ever wrote `watched` when it was true,
 * because every plain progress report sent false too.
 *
 * Writes studi3's progress row for stream 8 and nothing else; see `markable` in seed.ts
 * for why no other file reads it. The row is put back to unwatched in the `finally`.
 */
test("marks a lecture as watched and unmarks it again", async ({ page, playwright }) => {
  const user = users[markable.user];
  const card = () => page.locator("article.tum-live-stream").filter({ hasText: markable.lecture });
  // The eye on the thumbnail; the open menu has eyes of its own.
  const mark = () => card().locator(".tum-live-thumbnail i.fa-eye");

  try {
    await login(page, user, courseUrl(markable.course));
    await expect(card()).toBeVisible();
    await expect(mark()).toHaveCount(0);

    await card().getByTitle("Lecture options").click();
    await card().getByRole("button", { name: "Mark as watched" }).click();
    await expect(mark()).toBeVisible();

    // Stored, not only shown.
    await page.reload();
    await expect(mark()).toBeVisible();

    await card().getByTitle("Lecture options").click();
    await card().getByRole("button", { name: "Unmark as watched" }).click();
    await expect(mark()).toHaveCount(0);

    await page.reload();
    await expect(card()).toBeVisible();
    await expect(mark()).toHaveCount(0);
  } finally {
    const api = await apiAs(playwright, user);
    const token = await bearerToken(api);
    await api.patch(`/api/v2/progress/${markable.streamId}`, {
      data: { progress: 0, watched: false },
      headers: { Authorization: `Bearer ${token}` },
    });
    await api.dispose();
  }
});
