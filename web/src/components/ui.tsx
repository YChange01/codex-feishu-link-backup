import { BrandLogo } from "./BrandLogo";

export type NoticeTone = "good" | "warn" | "danger";

export function BrandLockup(props: { subtitle: string; compact?: boolean }) {
  return (
    <div className={props.compact ? "brand brand-compact" : "brand"}>
      <BrandLogo className="brand-mark" />
      <div>
        <div className="brand-name">Codex Feishu Link</div>
        <div className="brand-sub">{props.subtitle}</div>
      </div>
    </div>
  );
}

export function Toast(props: { tone: NoticeTone; message: string }) {
  return (
    <div className={`toast ${props.tone}`} role="status">
      {props.message}
    </div>
  );
}
