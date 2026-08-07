import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const acceptedException = {
  expiresAt: "2026-08-14T15:59:59Z",
  packages: new Map([
    ["react-router", "7.18.2"],
    ["react-router-dom", "7.18.2"],
  ]),
  advisories: new Set(["GHSA-qwww-vcr4-c8h2"]),
};

const npmCommand = process.platform === "win32" ? "npm.cmd" : "npm";
const npmArgs = [
  "audit",
  "--json",
  "--omit=dev",
  "--audit-level=high",
  "--registry=https://registry.npmjs.org",
];
const result = spawnSync(
  process.platform === "win32" ? process.env.ComSpec : npmCommand,
  process.platform === "win32" ? ["/d", "/s", "/c", `${npmCommand} ${npmArgs.join(" ")}`] : npmArgs,
  {
    cwd: fileURLToPath(new URL("../../apps/admin/", import.meta.url)),
    encoding: "utf8",
  },
);

if (result.error || ![0, 1].includes(result.status)) {
  console.error(result.stderr || result.error?.message || "npm audit failed without a report");
  process.exit(1);
}

let report;
try {
  report = JSON.parse(result.stdout);
} catch {
  console.error("npm audit did not return valid JSON.");
  process.exit(1);
}

if (report.error) {
  console.error(`npm audit failed: ${report.error.summary || report.error}`);
  process.exit(1);
}

const highOrCritical = Object.entries(report.vulnerabilities ?? {}).filter(([, vulnerability]) =>
  ["high", "critical"].includes(vulnerability.severity),
);

if (highOrCritical.length === 0) {
  console.log("npm audit: no high or critical production vulnerabilities.");
  process.exit(0);
}

if (Date.now() > Date.parse(acceptedException.expiresAt)) {
  console.error(`The React Router security exception expired at ${acceptedException.expiresAt}.`);
  process.exit(1);
}

const unexpectedPackages = highOrCritical
  .map(([name]) => name)
  .filter((name) => !acceptedException.packages.has(name));

const advisoryIds = new Set(
  highOrCritical.flatMap(([, vulnerability]) =>
    vulnerability.via
      .filter((item) => typeof item === "object" && ["high", "critical"].includes(item.severity))
      .map((item) => item.url?.split("/").pop())
      .filter(Boolean),
  ),
);
const unexpectedAdvisories = [...advisoryIds].filter(
  (advisory) => !acceptedException.advisories.has(advisory),
);

const lock = JSON.parse(
  readFileSync(new URL("../../apps/admin/package-lock.json", import.meta.url), "utf8"),
);
const versionMismatches = [...acceptedException.packages].filter(
  ([name, version]) => lock.packages?.[`node_modules/${name}`]?.version !== version,
);

if (
  unexpectedPackages.length > 0 ||
  unexpectedAdvisories.length > 0 ||
  advisoryIds.size !== acceptedException.advisories.size ||
  versionMismatches.length > 0
) {
  console.error("npm audit found vulnerabilities outside the approved exception.");
  console.error(
    JSON.stringify(
      {
        vulnerablePackages: highOrCritical.map(([name]) => name),
        advisoryIds: [...advisoryIds],
        unexpectedPackages,
        unexpectedAdvisories,
        versionMismatches: versionMismatches.map(([name, version]) => `${name}@${version}`),
      },
      null,
      2,
    ),
  );
  process.exit(1);
}

console.warn(
  `npm audit: accepted ${[...advisoryIds].join(", ")} for React Router 7.18.2 until ${acceptedException.expiresAt}.`,
);
