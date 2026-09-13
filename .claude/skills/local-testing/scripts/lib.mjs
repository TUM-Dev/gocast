// Shared plumbing for the browser scripts.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export function repoRoot() {
  let d = path.dirname(fileURLToPath(import.meta.url));
  while (d !== "/") {
    if (fs.existsSync(path.join(d, "go.mod")) && fs.existsSync(path.join(d, "frontend"))) return d;
    d = path.dirname(d);
  }
  throw new Error("could not find the repo root (looked for go.mod next to frontend/)");
}

// Playwright is a dependency of frontend/, and these scripts live outside it. The
// package is `@playwright/test`, not `playwright` -- importing the latter fails with
// ERR_MODULE_NOT_FOUND even though the browsers are installed.
export async function chromium() {
  const entry = path.join(repoRoot(), "frontend/node_modules/@playwright/test/index.mjs");
  if (!fs.existsSync(entry)) throw new Error(`playwright not installed; run: pnpm --dir ${repoRoot()}/frontend install --frozen-lockfile`);
  return (await import(entry)).chromium;
}

export const BASE = process.env.GOCAST_BASE || "http://localhost:8081";

// Every fixture account uses the password "password"; the login field is the users
// table's `email` column: admin, prof1, prof2, studi1, studi2, studi3.
export async function login(page, who = "admin") {
  await page.goto(`${BASE}/login`, { waitUntil: "networkidle" });
  const user = page.locator('input[name="username"], input[type="text"]').first();
  if (!(await user.count())) throw new Error("no login form at /login -- is the server up?");
  await user.fill(who);
  await page.locator('input[type="password"]').first().fill("password");
  await Promise.all([
    page.waitForLoadState("networkidle"),
    page.locator('button[type="submit"], input[type="submit"]').first().click(),
  ]);
  if (page.url().includes("/login")) throw new Error(`login as ${who} failed -- reseed with: make e2e_db DB_CONTAINER=mariadb-tumlive`);
}

// Pages worth looking at, all reachable with the starter fixture. Everything under
// /admin needs the login. There is no /schedule route -- /admin *is* the schedule
// page (see GetPageString in web/admin.go), and /schedule renders the 404 page,
// whose animated GIF also makes it diff against itself.
export const PAGES = [
  ["login", "/login"],
  ["home", "/"],
  ["about", "/about"],
  ["search", "/search?q=bier"],
  ["course", "/course/2022/S/brauereiwesen"],
  ["watch", "/w/brauereiwesen/2"],
  ["admin-schedule", "/admin"],
  ["admin-course", "/admin/course/1"],
  ["admin-create-course", "/admin/create-course"],
  ["admin-course-import", "/admin/course-import"],
  ["admin-users", "/admin/users"],
  ["admin-lecture-halls", "/admin/lectureHalls"],
  ["lecture-units", "/admin/units/1/1"],
  ["lecture-cut", "/admin/cut/1/1"],
];

export const SHOTS = path.join(repoRoot(), ".shots");
