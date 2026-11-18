// Capture the real control UI served by a booted BitRiver QEMU/OVMF appliance.
// The page uses its actual local API, not mocked responses or fabricated art.
// Run only against a disposable CI guest: no production secrets or user data.
import { createRequire } from "node:module";
import { readFile, mkdir } from "node:fs/promises";
import path from "node:path";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.BITRIVER_PLAYWRIGHT_MODULE);
const output = process.env.BITRIVER_CAPTURE_OUTPUT;
const base = process.env.BITRIVER_CAPTURE_URL;
const keyFile = process.env.BITRIVER_CAPTURE_KEY_FILE;
const chrome = process.env.BITRIVER_CHROME;
if (!output || !base || !keyFile || !chrome) throw new Error("Capture environment is incomplete");
const key = (await readFile(keyFile, "utf8")).trim();
if (!/^[a-f0-9]{32,128}$/i.test(key)) throw new Error("Operator key not found in disposable guest");

await mkdir(output, { recursive: true });
const browser = await chromium.launch({
  headless: true,
  executablePath: chrome,
  args: ["--no-sandbox", "--disable-dev-shm-usage", "--disable-gpu"]
});
try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1
  });
  // Capture the *unauthenticated* live sign-in page first, not a mock-up.
  await page.goto(base, { waitUntil: "domcontentloaded", timeout: 45000 });
  await page.getByRole("heading", { name: "Operator sign in" }).waitFor({ timeout: 45000 });
  await page.screenshot({
    path: path.join(output, "operator-login.png"),
    animations: "disabled"
  });

  // Authenticate with an ephemeral key read privately from this throwaway VM.
  // Never write credentials to screenshots, console output, or CI artifacts.
  await page.locator("#operator-key").fill(key);
  await page.getByRole("button", { name: "Sign in" }).click();
  await page.locator(".program-card__screen").waitFor({ timeout: 45000 });
  await page.getByText("Program Output", { exact: true }).waitFor({ timeout: 45000 });
  await page.waitForTimeout(1500);
  if (await page.locator(".alert").count()) {
    throw new Error("Live appliance dashboard reported an API error");
  }
  await page.screenshot({
    path: path.join(output, "appliance-dashboard.png"),
    animations: "disabled"
  });
  await page.locator(".source-grid").scrollIntoViewIfNeeded();
  await page.screenshot({
    path: path.join(output, "appliance-sources.png"),
    animations: "disabled"
  });
  console.log("Captured verified live appliance sign-in, dashboard, and sources UI.");
} finally {
  await browser.close();
}
