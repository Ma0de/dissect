"use client";
import { useEffect, useRef } from "react";
import { useModel } from "./Shell";
import { Button } from "./ui";
import { IconCheck, IconX } from "./icons";

const UPLOAD_STEPS = [{ key: "unpack", label: "解压", state: "running" }];
const PREP_STEPS = [{ key: "unpack", label: "解压", state: "running" }];

export default function TaskView() {
  const { task, uploadPct, stopping, stopTask, offline } = useModel();
  const logRef = useRef<HTMLDivElement>(null);
  const uploading = uploadPct !== null;
  const pct = uploading ? Math.round((uploadPct || 0) * 100) : Math.round((task?.pct || 0) * 100);
  const steps = uploading
    ? UPLOAD_STEPS
    : task?.steps && task.steps.length
      ? task.steps
      : PREP_STEPS;
  const logs = task?.logs || [];
  const current = offline
    ? offline
    : uploading
      ? "正在上传文件…"
      : stopping
        ? "正在停止任务…"
        : task?.current || "正在准备…";
  const done = !!task?.done;

  useEffect(() => {
    const el = logRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [logs.length, current]);

  return (
    <div className="mx-auto w-full max-w-[1300px] pt-[9vh]">
      <div className="grid grid-cols-1 items-stretch gap-8 md:grid-cols-[200px_minmax(0,1fr)] md:gap-x-8">
        <div>
          <div className="text-[72px] font-semibold leading-none tracking-[-2px] tabular-nums text-primary">
            {pct}%
          </div>
          <div className="relative mt-8">
            <div className="absolute bottom-6 left-[25px] top-6 w-px bg-secondary" />
            <div className="flex flex-col gap-8">
              {steps.map((s, i) => (
                <div key={`${s.key}-${i}`} className="relative flex items-center gap-6">
                  <StepDot state={s.state} />
                  <span className={stepLabelCls(s.state)}>
                    {s.label}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="flex min-h-0 flex-col">
          <div className="flex min-h-[490px] flex-1 flex-col overflow-hidden rounded-xl bg-primary shadow-xs ring-1 ring-secondary">
            <div ref={logRef} className="min-h-0 flex-1 overflow-y-auto px-7 py-5">
              {logs.length === 0 ? (
                <div className="text-sm text-quaternary">等待任务日志…</div>
              ) : (
                logs.map((l, i) => (
                  <div key={i} className="flex gap-4 py-[3px] font-mono text-[13px] leading-6">
                    <span className="shrink-0 tabular-nums text-quaternary">{l.time}</span>
                    <span className={`break-all ${levelCls(l.level)}`}>{l.msg}</span>
                  </div>
                ))
              )}
            </div>
          </div>
          <div className="mt-5 flex items-center justify-between gap-4 pb-2">
            <div className="min-w-0 truncate text-base font-medium text-tertiary">{current}</div>
            <Button variant={done ? "secondary" : "danger"} size="lg" className="shrink-0"
              disabled={uploading || stopping || !!offline} onClick={stopTask}>
              {done ? "关闭" : stopping ? "正在停止…" : "暂停并结束任务"}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function levelCls(level?: string): string {
  switch (level) {
    case "error": return "text-fg-error-primary";
    case "warn": return "text-utility-yellow-700";
    case "success": return "text-fg-success-primary";
    default: return "text-secondary";
  }
}

function stepLabelCls(state?: string): string {
  switch (state) {
    case "done": return "text-xl font-medium text-secondary";
    case "running": return "text-xl font-semibold text-fg-brand-primary";
    case "error": return "text-xl font-semibold text-fg-error-primary";
    case "skipped": return "text-xl font-medium text-quaternary line-through";
    default: return "text-xl font-medium text-quaternary";
  }
}

function StepDot({ state }: { state?: string }) {
  const base = "relative z-10 flex size-[52px] shrink-0 items-center justify-center rounded-full border-2 transition duration-100";
  if (state === "done") {
    return (
      <span className={`${base} border-brand-600 bg-brand-solid text-white`}>
        <IconCheck size={24} />
      </span>
    );
  }
  if (state === "running") {
    return (
      <span className={`${base} border-brand-500 bg-primary`}>
        <span className="size-3 animate-pulse rounded-full bg-brand-500" />
      </span>
    );
  }
  if (state === "error") {
    return (
      <span className={`${base} border-red-500 bg-error-primary text-red-500`}>
        <IconX size={24} />
      </span>
    );
  }
  if (state === "skipped") {
    return (
      <span className={`${base} border-secondary bg-primary`}>
        <span className="h-0.5 w-3 rounded bg-quaternary" />
      </span>
    );
  }
  return <span className={`${base} border-secondary bg-primary`} />;
}
