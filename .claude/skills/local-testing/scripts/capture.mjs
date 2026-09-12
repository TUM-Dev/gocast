// Screenshot every page in both themes and report horizontal overflow.
//
//   node .claude/skills/local-testing/scripts/capture.mjs <label> [path...]
//
// Writes .shots/<label>-<page>-<theme>.png. Overflow is printed, not just drawn:
// a page whose scrollWidth exceeds the viewport is a layout regression whatever it
// looks like, and it is the failure mode CSS changes produce most often.
import fs from "node:fs";
import { chromium, BASE, login, PAGES, SHOTS } from "./lib.mjs";

const label = process.argv[2];
if (!label) { console.error("usage: capture.mjs <label> [path...]"); process.exit(1); }
const pages = process.argv.length > 3 ? process.argv.slice(3).map((p) => [p.replace(/\W+/g, "-").replace(/^-|-$/g, "") || "root", p]) : PAGES;

fs.mkdirSync(SHOTS, { recursive: true });
const browser = await (await chromium()).launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();
await login(page);

let overflows = 0, errors = 0;
for (const [name, p] of pages) {
  for (const theme of ["light", "dark"]) {
    try {
      await page.goto(BASE + p, { waitUntil: "networkidle", timeout: 25000 });
      // Dark mode is a `dark` class on <html>, set per template and toggled by Alpine.
      await page.evaluate((t) => document.documentElement.classList.toggle("dark", t === "dark"), theme);
      await page.waitForTimeout(500);
      const m = await page.evaluate(() => ({
        doc: document.documentElement.scrollWidth,
        vw: document.documentElement.clientWidth,
        // The 404 and error templates render with HTTP 200, so the status code does
        // not give this away: a screenshot of an error page looks like a pass.
        err: /Error: \d{3}|This page does not exist|You are not allowed/.test(document.body.innerText),
      }));
      if (m.err) { errors++; console.log(`  !! ${name}/${theme}: ERROR PAGE (served with HTTP 200) -- wrong path, or not logged in?`); }
      if (m.doc > m.vw) { overflows++; console.log(`  OVERFLOW ${name}/${theme}: scrollWidth=${m.doc} > viewport=${m.vw}`); }
      await page.screenshot({ path: `${SHOTS}/${label}-${name}-${theme}.png` });
    } catch (e) {
      console.log(`  !! ${name}/${theme}: ${e.message.split("\n")[0]}`);
    }
  }
  console.log(`shot ${name}`);
}
await browser.close();
console.log(overflows ? `\n${overflows} page(s) overflow horizontally` : "\nno horizontal overflow");
if (errors) console.log(`${errors} capture(s) are error pages -- fix the path or the login before trusting any diff`);
