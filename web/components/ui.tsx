// Untitled UI v8 组件库 —— class 配方逆向自 untitledui.com/react/components 的渲染产物
// 令牌：globals.css @theme（与他们编译后的 CSS 变量同名同值）
"use client";
import React, { createContext, useContext, useEffect, useRef, useState } from "react";
import { IconCheck, IconCopy, IconSpin, IconChevronRight, IconX } from "./icons";

/* ---------------- Button（buttons 组件页原样配方） ---------------- */
type BtnVariant = "primary" | "secondary" | "tertiary" | "destructive" | "danger";
type BtnSize = "sm" | "md" | "lg";

const BTN_BASE =
  "group relative inline-flex h-max cursor-pointer items-center justify-center whitespace-nowrap " +
  "outline-focus-ring transition duration-100 ease-linear focus-visible:outline-2 focus-visible:outline-offset-2 " +
  "disabled:cursor-not-allowed disabled:opacity-50 gap-1.5 rounded-lg font-semibold select-none";
const BTN_SIZE: Record<BtnSize, string> = {
  sm: "px-2.5 py-1.5 text-sm [&_svg]:size-4 [&_svg]:stroke-[2.25px]",
  md: "px-3.5 py-2.5 text-sm [&_svg]:size-5",
  lg: "px-4.5 py-3 text-md [&_svg]:size-5",
};
const BTN_VARIANT: Record<BtnVariant, string> = {
  // 主按钮：brand-solid + skeuomorphic 内描边高光（v8 新增的 before 蒙版层）
  primary:
    "bg-brand-solid text-white shadow-xs-skeuomorphic ring-1 ring-transparent ring-inset hover:bg-brand-solid_hover " +
    "before:absolute before:inset-px before:border before:border-white/12 before:rounded-[7px] before:mask-b-from-0% " +
    "[&_svg]:text-white/60",
  secondary:
    "bg-primary text-secondary shadow-xs-skeuomorphic ring-1 ring-primary ring-inset hover:bg-primary_hover " +
    "hover:text-secondary_hover [&_svg]:text-fg-quaternary",
  tertiary:
    "bg-transparent text-fg-brand-primary hover:bg-brand-secondary/10 dark:hover:bg-brand-secondary/20",
  destructive:
    "bg-primary text-fg-error-primary shadow-xs-skeuomorphic ring-1 ring-primary ring-inset hover:bg-primary_hover " +
    "[&_svg]:text-fg-error-secondary",
  danger:
    "bg-error-solid text-white shadow-xs-skeuomorphic ring-1 ring-transparent ring-inset hover:bg-error-solid_hover " +
    "before:absolute before:inset-px before:border before:border-white/12 before:rounded-[7px] before:mask-b-from-0% " +
    "[&_svg]:text-white/60",
};

export function Button({
  variant = "secondary", size = "md", children, className = "", ...rest
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: BtnVariant; size?: BtnSize }) {
  return (
    <button className={`${BTN_BASE} ${BTN_SIZE[size]} ${BTN_VARIANT[variant]} ${className}`} {...rest}>
      {children}
    </button>
  );
}

/* ---------------- IconButton（utility-buttons 配方） ---------------- */
export function IconButton({ children, className = "", ...rest }: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      className={
        "group inline-flex h-max cursor-pointer items-center justify-center rounded-md p-1.5 outline-focus-ring " +
        "transition duration-100 ease-linear focus-visible:outline-2 focus-visible:outline-offset-2 " +
        "disabled:cursor-not-allowed disabled:opacity-50 text-fg-quaternary hover:bg-primary_hover " +
        "hover:text-fg-quaternary_hover [&_svg]:size-4 [&_svg]:stroke-[2.25px] " + className
      }
      {...rest}
    >
      {children}
    </button>
  );
}

/* ---------------- Badge（badges 组件页：utility-* 色自动翻转亮暗） ---------------- */
export type BadgeColor = "gray" | "brand" | "success" | "warning" | "error" | "blue";
const BADGE_COLOR: Record<BadgeColor, string> = {
  gray: "bg-primary text-secondary ring-primary",
  brand: "bg-utility-brand-50 text-utility-brand-700 ring-utility-brand-200",
  success: "bg-utility-green-50 text-utility-green-700 ring-utility-green-200",
  warning: "bg-utility-yellow-50 text-utility-yellow-700 ring-utility-yellow-200",
  error: "bg-utility-red-50 text-utility-red-700 ring-utility-red-200",
  blue: "bg-utility-blue-50 text-utility-blue-700 ring-utility-blue-200",
};
export function Badge({ color = "gray", dot, children, className = "" }: {
  color?: BadgeColor; dot?: string; children: React.ReactNode; className?: string;
}) {
  return (
    <span className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-md ring-1 ring-inset shadow-xs py-0.5 px-1.5 text-xs font-semibold ${BADGE_COLOR[color]} ${className}`}>
      {dot && <span className="size-1.5 rounded-full" style={{ background: dot }} />}
      {children}
    </span>
  );
}

/* ---------------- Card / Metric ---------------- */
export function Card({ title, extra, children, flush, className = "" }: {
  title?: React.ReactNode; extra?: React.ReactNode; children: React.ReactNode; flush?: boolean; className?: string;
}) {
  return (
    <section className={`rounded-xl bg-primary shadow-xs ring-1 ring-secondary ${className}`}>
      {(title || extra) && (
        <div className="flex items-center justify-between gap-3 border-b border-secondary px-5 py-3.5">
          <div className="text-md font-semibold text-primary">{title}</div>
          {extra}
        </div>
      )}
      <div className={flush ? "" : "p-5"}>{children}</div>
    </section>
  );
}

export function Metric({ label, value, hint, icon }: {
  label: string; value: React.ReactNode; hint?: string; icon?: React.ReactNode;
}) {
  return (
    <div className="rounded-xl bg-primary p-5 shadow-xs ring-1 ring-secondary">
      <div className="flex items-center gap-2 text-sm font-medium text-tertiary [&_svg]:size-4 [&_svg]:text-fg-quaternary">
        {icon}{label}
      </div>
      <div className="mt-1.5 text-3xl font-semibold text-primary">{value}</div>
      {hint && <div className="mt-1 text-xs font-medium text-quaternary">{hint}</div>}
    </div>
  );
}

/* ---------------- Input / SearchInput（inputs 组件页配方） ---------------- */
export function Input({ className = "", ...rest }: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={`m-0 w-full rounded-lg bg-transparent text-primary ring-0 outline-none placeholder:text-placeholder disabled:cursor-not-allowed ${className}`}
      {...rest}
    />
  );
}

export function SearchInput({ value, onChange, placeholder }: {
  value: string; onChange: (v: string) => void; placeholder?: string;
}) {
  return (
    <div className="group/input relative flex w-full items-center rounded-lg bg-primary shadow-xs ring-1 ring-primary ring-inset transition duration-100 ease-linear focus-within:ring-2 focus-within:ring-brand">
      <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round"
        className="pointer-events-none absolute left-3.5 size-4 text-fg-quaternary">
        <circle cx="9" cy="9" r="5.5" /><path d="m13.2 13.2 3.8 3.8" />
      </svg>
      <Input value={value} placeholder={placeholder} onChange={(e) => onChange(e.target.value)}
        className="h-10 pl-10 pr-3 text-sm" />
    </div>
  );
}

/* ---------------- Toggle（toggles 组件页配方） ---------------- */
export function Toggle({ checked, onChange, disabled }: {
  checked: boolean; onChange: (v: boolean) => void; disabled?: boolean;
}) {
  return (
    <button
      role="switch" aria-checked={checked} disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`shrink-0 cursor-pointer rounded-full transition duration-150 ease-linear ring-1 ring-inset h-5 w-9 p-0.5 ${
        checked ? "bg-brand-solid ring-transparent" : "bg-tertiary ring-secondary"
      } ${disabled ? "cursor-not-allowed opacity-50" : ""}`}
    >
      <span className={`block size-4 rounded-full bg-fg-white shadow-xs border border-toggle-border transition-transform duration-150 ${checked ? "translate-x-4" : ""}`} />
    </button>
  );
}

/* ---------------- Progress（progress-indicators 配方） ---------------- */
export function Progress({ pct, className = "" }: { pct: number; className?: string }) {
  return (
    <div className={`h-2 w-full overflow-hidden rounded-full bg-tertiary ${className}`}>
      <div className="size-full rounded-md bg-fg-brand-primary transition-[width] duration-75 ease-linear"
        style={{ width: `${Math.min(100, Math.max(0, pct))}%` }} />
    </div>
  );
}

/* ---------------- Tabs（下划线式，badges 页提取到的 selected 配方） ---------------- */
export function Tabs({ tabs, value, onChange }: {
  tabs: { key: string; label: string; count?: number }[];
  value: string; onChange: (k: string) => void;
}) {
  return (
    <div className="flex items-center gap-5 border-b border-secondary">
      {tabs.map((t) => {
        const active = t.key === value;
        return (
          <button key={t.key} onClick={() => onChange(t.key)}
            className={`z-10 flex h-max cursor-pointer items-center justify-center gap-1 whitespace-nowrap rounded-none border-b-2 px-0.5 pb-2.5 pt-0 text-sm font-semibold transition duration-100 ease-linear outline-focus-ring ${
              active ? "border-fg-brand-primary_alt text-primary" : "border-transparent text-tertiary hover:text-secondary"
            }`}>
            {t.label}
            {t.count !== undefined && (
              <span className={active ? "text-brand-secondary text-xs" : "text-quaternary text-xs"}>{t.count}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}

/* ---------------- Menu（下拉/右键菜单共用面板与条目） ---------------- */
export interface MenuItemSpec {
  label: string;
  icon?: React.ReactNode;
  shortcut?: string;
  danger?: boolean;
  disabled?: boolean;
  onClick?: () => void;
}

export function MenuPanel({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`z-50 min-w-[220px] rounded-xl bg-primary p-1 shadow-lg ring-1 ring-secondary animate-pop-in ${className}`}>
      {children}
    </div>
  );
}

export function MenuItem({ item }: { item: MenuItemSpec }) {
  return (
    <button
      disabled={item.disabled}
      onClick={item.onClick}
      className={`flex w-full cursor-pointer select-none items-center gap-2 rounded-md px-2.5 py-2 text-sm font-semibold outline-none transition duration-100 ease-linear disabled:cursor-default disabled:opacity-40 ${
        item.danger ? "text-fg-error-primary hover:bg-error-primary" : "text-secondary hover:bg-primary_hover"
      }`}
    >
      {item.icon && <span className="[&_svg]:size-4 [&_svg]:stroke-[2.25px]">{item.icon}</span>}
      <span className="flex-1 text-left">{item.label}</span>
      {item.shortcut && <span className="text-xs font-medium text-quaternary">{item.shortcut}</span>}
    </button>
  );
}

export function MenuSeparator() {
  return <div className="my-1 border-t border-secondary" />;
}

/* ---------------- ContextMenu（右键菜单：context-menus 组件） ---------------- */
interface CtxState { x: number; y: number; items: MenuItemSpec[]; }
const CtxCtx = createContext<(e: React.MouseEvent, items: MenuItemSpec[]) => void>(() => {});
export const useContextMenu = () => useContext(CtxCtx);

export function ContextMenuProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<CtxState | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!state) return;
    const close = () => setState(null);
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setState(null);
    };
    const onKey = (e: KeyboardEvent) => { if (e.key === "Escape") setState(null); };
    document.addEventListener("click", close);
    document.addEventListener("scroll", close, true);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("click", close);
      document.removeEventListener("scroll", close, true);
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [state]);
  const open = (e: React.MouseEvent, items: MenuItemSpec[]) => {
    e.preventDefault();
    e.stopPropagation();
    const W = 240, H = Math.min(items.length, 9) * 36 + 8;
    const x = Math.min(e.clientX, window.innerWidth - W - 8);
    const y = Math.min(e.clientY, window.innerHeight - H - 8);
    setState({ x, y, items });
  };
  return (
    <CtxCtx.Provider value={open}>
      {children}
      {state && (
        <div ref={ref} className="fixed" style={{ left: state.x, top: state.y }}>
          <MenuPanel>
            {state.items.map((it, i) => <MenuItem key={i} item={it} />)}
          </MenuPanel>
        </div>
      )}
    </CtxCtx.Provider>
  );
}

/* ---------------- Breadcrumb（breadcrumbs 组件页配方） ---------------- */
export function Breadcrumb({ items }: { items: { label: string; onClick?: () => void }[] }) {
  return (
    <nav className="flex items-center gap-0.5">
      {items.map((it, i) => {
        const last = i === items.length - 1;
        return (
          <React.Fragment key={i}>
            {i > 0 && <IconChevronRight size={14} className="text-fg-quaternary" />}
            <button
              onClick={it.onClick}
              className={`flex cursor-pointer items-center gap-1 rounded-md py-1 pr-1.5 pl-2 text-sm font-semibold outline-none transition duration-100 ease-linear ${
                last ? "text-primary" : "text-tertiary hover:bg-primary_hover"
              }`}
            >
              {it.label}
            </button>
          </React.Fragment>
        );
      })}
    </nav>
  );
}

/* ---------------- EmptyState + FeaturedIcon ---------------- */
export function FeaturedIcon({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`relative flex shrink-0 items-center justify-center bg-primary shadow-xs-skeuomorphic ring-1 ring-primary ring-inset size-12 rounded-[10px] text-fg-secondary [&_svg]:size-6 ${className}`}>
      {children}
    </div>
  );
}

export function EmptyState({ icon, title, sub, actions, className = "" }: {
  icon?: React.ReactNode; title: string; sub?: React.ReactNode;
  actions?: React.ReactNode; className?: string;
}) {
  return (
    <div className={`flex flex-col items-center gap-4 text-center ${className}`}>
      {icon && <FeaturedIcon>{icon}</FeaturedIcon>}
      <div>
        <div className="text-lg font-semibold text-primary">{title}</div>
        {sub && <div className="mt-1.5 text-sm text-tertiary">{sub}</div>}
      </div>
      {actions && <div className="mt-1 flex flex-wrap items-center justify-center gap-3">{actions}</div>}
    </div>
  );
}

/* ---------------- Dropzone（file-uploaders 配方） ---------------- */
export function Dropzone({ dragging, children, className = "" }: {
  dragging: boolean; children: React.ReactNode; className?: string;
}) {
  return (
    <div
      className={`relative flex flex-col items-center gap-3 rounded-xl bg-primary px-6 py-8 text-tertiary transition duration-100 ease-linear ring-inset ${
        dragging ? "ring-2 ring-brand" : "ring-1 ring-secondary"
      } ${className}`}
    >
      {children}
    </div>
  );
}

/* ---------------- Alert（alerts 组件页配方） ---------------- */
export function Alert({ type = "info", title, children, onClose }: {
  type?: "info" | "success" | "error" | "warning";
  title: string; children?: React.ReactNode; onClose?: () => void;
}) {
  const skin = {
    info: "bg-primary ring-secondary text-primary",
    success: "bg-success-primary ring-utility-green-200",
    error: "bg-error-primary ring-utility-red-200",
    warning: "bg-warning-primary ring-utility-yellow-200",
  }[type];
  const fg = {
    info: "text-primary", success: "text-utility-green-700", error: "text-utility-red-700", warning: "text-utility-yellow-700",
  }[type];
  return (
    <div className={`relative flex gap-3 rounded-xl p-4 ring-1 ring-inset transition-shadow duration-100 ease-linear ${skin}`}>
      <div className="mt-0.5 shrink-0 [&_svg]:size-5">
        {type === "success" ? <IconCheck className={fg} /> : type === "error" ? <IconX className={fg} /> : null}
      </div>
      <div className="min-w-0 flex-1">
        <div className={`text-sm font-semibold ${type === "info" ? "text-primary" : fg}`}>{title}</div>
        {children && <div className="mt-1 text-sm font-medium text-secondary break-all">{children}</div>}
      </div>
      {onClose && (
        <button onClick={onClose} className="absolute right-2.5 top-2.5 cursor-pointer text-quaternary hover:text-secondary">
          <IconX size={16} />
        </button>
      )}
    </div>
  );
}

/* ---------------- CodeSnippet（code-snippets 配方） ---------------- */
export function CodeSnippet({ code, className = "", maxH = "max-h-[480px]" }: {
  code: string; className?: string; maxH?: string;
}) {
  return (
    <div className={`group/code-snippet relative max-w-full overflow-clip rounded-xl bg-primary ring-1 ring-secondary ${className}`}>
      <pre className={`overflow-auto p-5 font-mono text-sm font-medium leading-[24px] text-primary ${maxH}`}>
        {code}
      </pre>
    </div>
  );
}

/* ---------------- ButtonGroup（button-groups 配方） ---------------- */
export function ButtonGroup({ options, value, onChange }: {
  options: { key: string; label: string; icon?: React.ReactNode }[];
  value: string; onChange: (k: string) => void;
}) {
  return (
    <div className="relative z-0 inline-flex w-max -space-x-px rounded-lg shadow-xs">
      {options.map((o, i) => {
        const active = o.key === value;
        return (
          <button key={o.key} onClick={() => onChange(o.key)}
            className={`relative z-0 inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-semibold transition duration-100 ease-linear outline-focus-ring ring-1 ring-inset ${
              i === 0 ? "rounded-l-lg" : ""
            } ${i === options.length - 1 ? "rounded-r-lg" : ""} ${
              active
                ? "z-10 bg-secondary_alt text-primary ring-primary"
                : "bg-primary text-secondary ring-primary hover:bg-primary_hover"
            }`}>
            {o.icon && <span className="[&_svg]:size-4 [&_svg]:text-fg-quaternary">{o.icon}</span>}
            {o.label}
          </button>
        );
      })}
    </div>
  );
}

/* ---------------- Modal / Loading / Avatar / CopyBtn ---------------- */
export function Modal({ title, sub, actions, children, onClose }: {
  title: string; sub?: string; actions?: React.ReactNode; children?: React.ReactNode; onClose: () => void;
}) {
  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-alpha-black/40 p-4 dark:bg-black/70"
      onClick={onClose}>
      <div className="w-full max-w-md rounded-xl bg-primary p-6 shadow-lg ring-1 ring-secondary animate-pop-in"
        onClick={(e) => e.stopPropagation()}>
        <div className="text-lg font-semibold text-primary">{title}</div>
        {sub && <div className="mt-1.5 text-sm text-tertiary">{sub}</div>}
        {children}
        {actions && <div className="mt-5 flex justify-end gap-3">{actions}</div>}
      </div>
    </div>
  );
}

export function Loading({ text = "加载中…" }: { text?: string }) {
  return (
    <div className="flex flex-col items-center gap-3 py-16 text-tertiary">
      <span className="text-brand-500"><IconSpin /></span>
      <span className="text-sm font-medium">{text}</span>
    </div>
  );
}

export function Spinner({ size = 18, className = "" }: { size?: number; className?: string }) {
  return (
    <span className={`inline-flex shrink-0 text-brand-500 ${className}`}>
      <IconSpin size={size} />
    </span>
  );
}

export function ProgressBar({ pct, className = "" }: { pct: number; className?: string }) {
  const v = Math.max(0, Math.min(1, pct));
  return (
    <div className={`h-2 w-full overflow-hidden rounded-full bg-tertiary ${className}`}>
      <div className="h-full rounded-full bg-brand-500 transition-[width] duration-150 ease-linear"
        style={{ width: `${v * 100}%` }} />
    </div>
  );
}

export function Avatar({ name, color }: { name: string; color?: string }) {
  const h = [...name].reduce((a, c) => a + c.charCodeAt(0), 0);
  const palette = ["#7f56d9", "#2e90fa", "#12b76a", "#f79009", "#ee46bc", "#16b364", "#6172f3", "#dd2590"];
  const bg = color || palette[h % palette.length];
  return (
    <div className="relative inline-flex size-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold text-white"
      style={{ background: bg }}>
      {[...name].slice(0, 1).join("").toUpperCase()}
    </div>
  );
}

export function CopyBtn({ text, label = "复制" }: { text: string; label?: string }) {
  const [done, setDone] = useState(false);
  return (
    <Button size="sm" variant="secondary"
      onClick={async () => {
        try { await navigator.clipboard.writeText(text); } catch { /* ignore */ }
        setDone(true);
        setTimeout(() => setDone(false), 1500);
      }}>
      {done ? <IconCheck /> : <IconCopy />}
      {done ? "已复制" : label}
    </Button>
  );
}

export function kindBadgeColor(kind: string): BadgeColor {
  switch (kind) {
    case "image": return "brand";
    case "audio": return "success";
    case "font": return "warning";
    case "script": return "blue";
    case "video": return "error";
    default: return "gray";
  }
}

/* ---------------- PageHead（页头：display-xs 标题 + 行内操作） ---------------- */
export function PageHead({ title, sub, actions }: {
  title: string; sub?: React.ReactNode; actions?: React.ReactNode;
}) {
  return (
    <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
      <div>
        <h1 className="text-display-xs font-semibold tracking-[-0.36px] text-primary">{title}</h1>
        {sub && <div className="mt-1 text-sm text-tertiary">{sub}</div>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-3">{actions}</div>}
    </div>
  );
}

/* ---------------- 表格（对齐他们的 table 规范：secondary_alt 表头 + 分隔行） ---------------- */
export function Tbl({ cols, children, className = "" }: {
  cols: React.ReactNode[]; children: React.ReactNode; className?: string;
}) {
  return (
    <div className={`overflow-x-auto ${className}`}>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="bg-secondary_alt">
            {cols.map((c, i) => (
              <th key={i} className="whitespace-nowrap border-b border-secondary px-4 py-2.5 text-left text-xs font-semibold text-tertiary">
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}

export function Tr({ children }: { children: React.ReactNode }) {
  return <tr className="transition-colors duration-100 ease-linear hover:bg-primary_hover">{children}</tr>;
}

export function Td({ children, mono, dim, className = "" }: {
  children: React.ReactNode; mono?: boolean; dim?: boolean; className?: string;
}) {
  return (
    <td className={`border-b border-secondary px-4 py-2.5 align-middle ${
      mono ? "font-mono text-xs" : ""
    } ${dim ? "text-quaternary" : "text-secondary"} ${className}`}>
      {children}
    </td>
  );
}

/* ---------------- LoadMoreBtn ---------------- */
export function LoadMore({ onClick, children }: { onClick: () => void; children: React.ReactNode }) {
  return (
    <div className="py-3.5 text-center">
      <Button variant="secondary" size="sm" onClick={onClick}>{children}</Button>
    </div>
  );
}

/* ---------------- TreeView（tree-views 组件页配方：guide 线 + chevron 旋转） ---------------- */
export function TreeChevron({ open }: { open: boolean }) {
  return (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth={3} strokeLinecap="round"
      strokeLinejoin="round"
      className={`size-3 shrink-0 text-fg-quaternary transition duration-150 ease-out ${open ? "" : "-rotate-90"}`}>
      <path d="m4.5 7.5 5.5 5.5 5.5-5.5" />
    </svg>
  );
}
