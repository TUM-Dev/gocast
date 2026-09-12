// Pixel-diff two capture runs.
//
//   node .claude/skills/local-testing/scripts/compare.mjs <baseline> <candidate>
//
// Prints the share of differing pixels per page, worst first, and flags size
// mismatches separately -- those mean the layout moved, not just the colours.
import fs from "node:fs";
import { chromium, SHOTS } from "./lib.mjs";

const [a, b] = process.argv.slice(2);
if (!a || !b) { console.error("usage: compare.mjs <baseline> <candidate>"); process.exit(1); }
const names = fs.readdirSync(SHOTS).filter((f) => f.startsWith(`${a}-`)).map((f) => f.slice(a.length + 1, -4));
if (!names.length) { console.error(`no shots for "${a}" in ${SHOTS}`); process.exit(1); }

const browser = await (await chromium()).launch();
const page = await browser.newPage();
await page.goto("about:blank");
const rows = [];
for (const n of names) {
  const fa = `${SHOTS}/${a}-${n}.png`, fb = `${SHOTS}/${b}-${n}.png`;
  if (!fs.existsSync(fb)) { rows.push([n, -1, `missing ${b}-${n}.png`]); continue; }
  const r = await page.evaluate(async ([da, db]) => {
    const load = (d) => new Promise((res) => { const i = new Image(); i.onload = () => res(i); i.src = d; });
    const [ia, ib] = await Promise.all([load(da), load(db)]);
    if (ia.width !== ib.width || ia.height !== ib.height) return { size: `${ia.width}x${ia.height} vs ${ib.width}x${ib.height}` };
    const data = (im) => { const c = document.createElement("canvas"); c.width = im.width; c.height = im.height;
      const g = c.getContext("2d"); g.drawImage(im, 0, 0); return g.getImageData(0, 0, c.width, c.height).data; };
    const pa = data(ia), pb = data(ib);
    let d = 0;
    for (let i = 0; i < pa.length; i += 4)
      if (Math.abs(pa[i] - pb[i]) > 8 || Math.abs(pa[i + 1] - pb[i + 1]) > 8 || Math.abs(pa[i + 2] - pb[i + 2]) > 8) d++;
    return { pct: (d / (pa.length / 4)) * 100 };
  }, [`data:image/png;base64,${fs.readFileSync(fa).toString("base64")}`, `data:image/png;base64,${fs.readFileSync(fb).toString("base64")}`]);
  rows.push(r.size ? [n, 1e9, `SIZE MISMATCH ${r.size} -- layout moved`] : [n, r.pct, `${r.pct.toFixed(2)}%`]);
}
rows.sort((x, y) => y[1] - x[1]);
for (const [n, , text] of rows) console.log(`  ${n.padEnd(24)} ${text}`);
await browser.close();
