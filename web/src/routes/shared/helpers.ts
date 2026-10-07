import { type APIErrorShape, formatError, requestJSON } from "../../lib/api";
import type { AutostartDetectResponse, VSCodeDetectResponse } from "../../lib/types";

export type VSCodeUsageScenario = "current_machine" | "remote_only";

export function blankToUndefined(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed ? trimmed : undefined;
}

export function readAPIError(response: { ok: boolean; data: unknown }) {
  if (response.ok) {
    return null;
  }
  const payload = response.data as APIErrorShape;
  return payload.error || null;
}

export async function loadVSCodeState(
  path: string,
  timeoutMs?: number,
): Promise<{ data: VSCodeDetectResponse | null; error: string }> {
  try {
    return {
      data: await requestJSON<VSCodeDetectResponse>(path, undefined, { timeoutMs }),
      error: "",
    };
  } catch (err: unknown) {
    return {
      data: null,
      error: formatError(err),
    };
  }
}

export async function loadAutostartState(path: string): Promise<{ data: AutostartDetectResponse | null; error: string }> {
  try {
    return {
      data: await requestJSON<AutostartDetectResponse>(path),
      error: "",
    };
  } catch {
    return {
      data: null,
      error: "自动运行状态暂时无法读取，请稍后重试。",
    };
  }
}

export function vscodeIsReady(vscode: VSCodeDetectResponse | null): boolean {
  if (!vscode) {
    return false;
  }
  return (
    vscode.latestShim.kind === "tiny_shim" &&
    vscode.latestShim.installed &&
    vscode.latestShim.sidecarValid &&
    !vscode.needsShimReinstall &&
    !vscode.settings.matchesBinary
  );
}

export function vscodeApplyModeForScenario(vscode: VSCodeDetectResponse | null, scenario: VSCodeUsageScenario | null): string | null {
  if (!vscode) {
    return null;
  }
  if (vscode.sshSession) {
    return "managed_shim";
  }
  switch (scenario) {
    case "current_machine":
      return "managed_shim";
    default:
      return null;
  }
}
