// 选项分支页
"use client";
import { useMemo, useState } from "react";
import Link from "next/link";
import { useModel } from "../../components/Shell";
import {
  Card, Loading, EmptyState, SearchInput, Badge, PageHead, Avatar, Button,
} from "../../components/ui";
import { exportURL } from "../../lib/api";
import { IconCopy } from "../../components/icons";

export default function Choices() {
  const { model, modelErr: err, addToast } = useModel();
  const [q, setQ] = useState("");

  const choices = useMemo(() => {
    if (!model) return [];
    let cs = model.choices;
    if (q.trim()) {
      const kw = q.trim().toLowerCase();
      cs = cs.filter((c) => c.text.toLowerCase().includes(kw) || (c.target || "").toLowerCase().includes(kw));
    }
    return cs;
  }, [model, q]);

  if (err) return <EmptyState title={`读取失败：${err}`} />;
  if (!model) return <Loading />;

  // 按 label 分组展示
  const groups: Record<string, typeof model.choices> = {};
  for (const c of choices) {
    (groups[c.label] ||= []).push(c);
  }

  return (
    <>
      <PageHead
        title="选项分支"
        sub={`${model.choices.length} 个选项 · ${model.flow.edges.filter((e) => e.kind === "choice").length} 条分支边`}
        actions={<Button variant="secondary" size="sm" onClick={() => location.assign(exportURL("choices.csv"))}>导出 CSV</Button>}
      />
      <div className="mb-4 max-w-[480px]">
        <SearchInput value={q} onChange={setQ} placeholder="搜索选项文本 / 跳转目标…" />
      </div>
      {Object.keys(groups).length === 0 && <EmptyState title="没有匹配的选项" />}
      {Object.entries(groups).map(([label, cs]) => (
        <div key={label} className="mb-4">
          <Card title={<span>菜单位于 <span className="font-mono text-sm">{label}</span></span>}
            extra={<span className="text-xs font-medium text-quaternary">{cs.length} 个选项</span>}>
            <div className="flex flex-col gap-2.5">
              {cs.map((c) => (
                <div key={c.id}
                  className="flex flex-wrap items-center gap-3 rounded-lg bg-primary p-3 ring-1 ring-secondary ring-inset transition duration-100 ease-linear hover:bg-primary_hover">
                  <Avatar name={c.text} />
                  <div className="min-w-[180px] flex-1">
                    <div className="text-sm font-medium text-primary">{c.text}</div>
                    <div className="mt-0.5 text-xs text-quaternary">
                      <span className="font-mono">{c.file}:{c.line}</span>
                      {c.cond && <> · 条件 <span className="font-mono">{c.cond}</span></>}
                    </div>
                  </div>
                  <Badge color={c.targetKind === "call" ? "warning" : c.targetKind === "call-screen" ? "blue" : "success"}>
                    {c.targetKind || "inline"} → {c.target || "(原地继续)"}
                  </Badge>
                  <div className="flex items-center gap-1.5">
                    <button
                      className="inline-flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 text-sm font-semibold text-tertiary transition duration-100 ease-linear hover:bg-primary_hover"
                      onClick={() => {
                        navigator.clipboard?.writeText(c.text).catch(() => {});
                        addToast("success", "选项文本已复制");
                      }}>
                      <IconCopy size={14} /> 复制
                    </button>
                    <Link className="inline-flex items-center rounded-md px-2 py-1 text-sm font-semibold text-fg-brand-secondary transition duration-100 ease-linear hover:bg-primary_hover"
                      href={`/flow?focus=${encodeURIComponent("L:" + (c.target || c.label))}`}>
                      在流程图中查看
                    </Link>
                    <Link className="inline-flex items-center rounded-md px-2 py-1 text-sm font-semibold text-tertiary transition duration-100 ease-linear hover:bg-primary_hover hover:text-secondary"
                      title={`在脚本中定位 ${c.file}:${c.line}`}
                      href={`/script?file=${encodeURIComponent(c.file)}&line=${c.line}`}>
                      源码
                    </Link>
                  </div>
                </div>
              ))}
            </div>
          </Card>
        </div>
      ))}
    </>
  );
}
