import { Config } from "@remotion/cli/config";
import { execSync } from "node:child_process";
import { existsSync } from "node:fs";

// Remotion's bundled Chrome download fails to extract on this machine, so use
// whatever headless shell or Chrome is already installed. Override with
// REMOTION_BROWSER if neither is where we look.
const browser =
  process.env.REMOTION_BROWSER ??
  (() => {
    try {
      const found = execSync(
        "find $HOME/Library/Caches/ms-playwright -name chrome-headless-shell -type f 2>/dev/null | head -1",
        { encoding: "utf8" },
      ).trim();
      if (found) return found;
    } catch {}
    const chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";
    return existsSync(chrome) ? chrome : null;
  })();

if (browser) Config.setBrowserExecutable(browser);

Config.setVideoImageFormat("jpeg");
Config.setOverwriteOutput(true);
// The terminal is flat colour and sharp text, so quality holds up cheaply.
Config.setCrf(18);
