#!/usr/bin/env node
/**
 * Postinstall script: downloads the platform-specific evo-cli Go binary
 * from the GitHub Release matching the package version.
 */
const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const https = require("https");

const pkg = require("../package.json");
const version = pkg.version;
const repo = "evopayment/evo-cli";
const binDir = path.join(__dirname, "..", "bin");

const platformMap = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};
const archMap = {
  x64: "amd64",
  arm64: "arm64",
};

const platform = platformMap[process.platform];
const arch = archMap[process.arch];

if (!platform || !arch) {
  console.error(`Unsupported platform: ${process.platform}/${process.arch}`);
  process.exit(1);
}

const ext = platform === "windows" ? "zip" : "tar.gz";
const assetName = `evo-cli_${version}_${platform}_${arch}.${ext}`;
const url = `https://github.com/${repo}/releases/download/v${version}/${assetName}`;

console.log(`Downloading evo-cli v${version} for ${platform}/${arch}...`);
console.log(`URL: ${url}`);

if (!fs.existsSync(binDir)) {
  fs.mkdirSync(binDir, { recursive: true });
}

const tmpFile = path.join(binDir, assetName);

function download(url, dest, cb) {
  const file = fs.createWriteStream(dest);
  https
    .get(url, (res) => {
      if (res.statusCode === 302 || res.statusCode === 301) {
        download(res.headers.location, dest, cb);
        return;
      }
      if (res.statusCode !== 200) {
        cb(new Error(`HTTP ${res.statusCode} downloading ${url}`));
        return;
      }
      res.pipe(file);
      file.on("finish", () => file.close(cb));
    })
    .on("error", cb);
}

download(url, tmpFile, (err) => {
  if (err) {
    console.error(`Failed to download: ${err.message}`);
    console.error("You can manually download from:", url);
    process.exit(1);
  }

  try {
    if (ext === "tar.gz") {
      execSync(`tar -xzf "${tmpFile}" -C "${binDir}" evo-cli`, {
        stdio: "inherit",
      });
    } else {
      execSync(`unzip -o "${tmpFile}" evo-cli.exe -d "${binDir}"`, {
        stdio: "inherit",
      });
    }
    fs.unlinkSync(tmpFile);

    const binary = path.join(binDir, platform === "windows" ? "evo-cli.exe" : "evo-cli");
    if (platform !== "windows") {
      fs.chmodSync(binary, 0o755);
    }
    console.log(`evo-cli installed to ${binary}`);
  } catch (e) {
    console.error(`Failed to extract: ${e.message}`);
    process.exit(1);
  }
});
