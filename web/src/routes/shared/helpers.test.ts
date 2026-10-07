import { describe, expect, it } from "vitest";
import { vscodeIsReady } from "./helpers";
import { makeVSCodeDetect } from "../../test/fixtures";

describe("shared vscode helpers", () => {
  it("treats legacy settings residue as not ready even if VS Code integration is installed", () => {
    const detect = makeVSCodeDetect({
      settings: {
        path: "/tmp/settings.json",
        exists: true,
        cliExecutable: "/usr/local/bin/codex-feishu-relay",
        matchesBinary: true,
      },
      latestShim: {
        entrypoint: "/tmp/codex-shim.js",
        exists: true,
        kind: "tiny_shim",
        repoManaged: true,
        realBinaryPath: "/usr/local/bin/codex",
        realBinaryExists: true,
        sidecarPath: "/tmp/codex-shim.js.codex-feishu-relay.json",
        sidecarExists: true,
        sidecarValid: true,
        installed: true,
        matchesBinary: true,
      },
      needsShimReinstall: false,
    });

    expect(vscodeIsReady(detect)).toBe(false);
  });

  it("treats an old but valid VS Code integration as ready", () => {
    const detect = makeVSCodeDetect({
      latestShim: {
        ...makeVSCodeDetect().latestShim,
        matchesBinary: false,
      },
      needsShimReinstall: false,
    });

    expect(vscodeIsReady(detect)).toBe(true);
  });

  it("treats a broken VS Code integration as needing repair", () => {
    const detect = makeVSCodeDetect({
      latestShim: {
        ...makeVSCodeDetect().latestShim,
        installed: false,
        sidecarValid: false,
      },
      needsShimReinstall: true,
    });

    expect(vscodeIsReady(detect)).toBe(false);
  });
});
