#!/usr/bin/env node
/**
 * npx entry point: forwards process.argv to the Go binary.
 */
const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const binName = process.platform === "win32" ? "evo-cli.exe" : "evo-cli";
const binPath = path.join(__dirname, "..", "bin", binName);

if (!fs.existsSync(binPath)) {
  console.error(
    "evo-cli binary not found. Run 'npm install' or 'node scripts/install.js' first."
  );
  process.exit(1);
}

try {
  execFileSync(binPath, process.argv.slice(2), { stdio: "inherit" });
} catch (e) {
  process.exit(e.status || 1);
}
