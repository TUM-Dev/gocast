// List every CSS rule setting a property on an element, with its cascade layer.
//
//   node .claude/skills/local-testing/scripts/matched-rules.mjs <path> <selector> <property>
//   node .claude/skills/local-testing/scripts/matched-rules.mjs /w/brauereiwesen/2 video-js.video-js width
//
// Use when an element is the wrong size or colour and the utility class that should
// control it looks correct. The `layer` column is the answer more often than
// specificity: an unlayered rule beats a layered one no matter the order.
import { chromium, BASE, login } from "./lib.mjs";

const [path_, selector, prop] = process.argv.slice(2);
if (!path_ || !selector || !prop) { console.error("usage: matched-rules.mjs <path> <selector> <property>"); process.exit(1); }

const browser = await (await chromium()).launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await ctx.newPage();
await login(page);
await page.goto(BASE + path_, { waitUntil: "networkidle" });
await page.waitForTimeout(900);

const cdp = await ctx.newCDPSession(page);
await cdp.send("DOM.enable"); await cdp.send("CSS.enable");
const { root } = await cdp.send("DOM.getDocument", { depth: -1, pierce: true });
const { nodeId } = await cdp.send("DOM.querySelector", { nodeId: root.nodeId, selector });
if (!nodeId) { console.error(`no element matches ${selector}`); process.exit(1); }
const m = await cdp.send("CSS.getMatchedStylesForNode", { nodeId });

console.log(`rules setting \`${prop}\` on ${selector}, weakest first:\n`);
for (const r of m.matchedCSSRules || []) {
  const d = (r.rule.style.cssProperties || []).find((p) => p.name === prop);
  if (d) console.log(`  ${r.rule.selectorList.text.slice(0, 60).padEnd(62)} ${prop}:${String(d.value).padEnd(10)} layer=${(r.rule.layers || []).map((l) => l.text).join(">") || "(none)"}`);
}
console.log(`\ncomputed: ${await page.evaluate(([s, p]) => getComputedStyle(document.querySelector(s))[p], [selector, prop])}`);
await browser.close();
