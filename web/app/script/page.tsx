"use client";
import { useEffect, useMemo, useRef, useState } from "react";
import { useModel } from "../../components/Shell";
import { Card, EmptyState, Loading, PageHead, SearchInput, Button } from "../../components/ui";
import { IconCopy } from "../../components/icons";

interface ScriptMeta {
  name: string;
  lines: number;
  size: number;
}

export default function ScriptPage() {
  const { addToast } = useModel();
  const [files, setFiles] = useState<ScriptMeta[] | null>(null);
  const [q, setQ] = useState("");
  const [name, setName] = useState("");
  const [target, setTarget] = useState(0);
  const [content, setContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState("");
  const codeRef = useRef<HTMLDivElement>(null);
  const started = useRef(false);

  const open = async (n: string, line = 0) => {
    if (!n) return;
    setName(n);
    setTarget(line);
    setLoading(true);
    setErr("");
    try {
      const r = await fetch(`/api/script?name=${encodeURIComponent(n)}`, { cache: "no-store" });
      if (!r.ok) {
        const j = await r.json().catch(() => null);
        throw new Error((j && j.error) || `读取失败 (${r.status})`);
      }
      setContent(await r.text());
      window.history.replaceState(null, "", `/script?file=${encodeURIComponent(n)}${line > 0 ? `&line=${line}` : ""}`);
    } catch (e: any) {
      setContent(null);
      setErr(e?.message || String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const r = await fetch("/api/scripts", { cache: "no-store" });
        const j = await r.json().catch(() => null);
        if (!alive) return;
        const list: ScriptMeta[] = Array.isArray(j) ? j : [];
        setFiles(list);
        if (!started.current) {
          started.current = true;
          const params = new URLSearchParams(window.location.search);
          const want = params.get("file") || "";
          const line = parseInt(params.get("line") || "0", 10) || 0;
          const hit = list.some((x) => x.name === want);
          const pick = hit ? want : list[0]?.name;
          if (pick) open(pick, hit ? line : 0);
        }
      } catch (e: any) {
        if (alive) setErr(e?.message || String(e));
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  const lines = useMemo(() => (content == null ? [] : content.split("\n")), [content]);

  useEffect(() => {
    if (!content) return;
    const box = codeRef.current;
    if (!box) return;
    if (!target) {
      box.scrollTop = 0;
      return;
    }
    const el = box.querySelector(`[data-line="${target}"]`) as HTMLElement | null;
    if (!el) return;
    const boxRect = box.getBoundingClientRect();
    const elRect = el.getBoundingClientRect();
    box.scrollTop += elRect.top - boxRect.top - box.clientHeight / 2 + elRect.height / 2;
  }, [content, target]);

  const shown = useMemo(() => {
    const list = files || [];
    const kw = q.trim().toLowerCase();
    return kw ? list.filter((f) => f.name.toLowerCase().includes(kw)) : list;
  }, [files, q]);

  return (
    <>
      <PageHead
        title="脚本"
        sub={files ? `${files.length} 个反编译脚本 · 从台词/选项的“定位”可以直接跳到对应行` : "加载中…"}
        actions={
          name ? (
            <Button variant="secondary" size="sm" onClick={() => {
              const s = target > 0 ? `${name}:${target}` : name;
              navigator.clipboard?.writeText(s).catch(() => {});
              addToast("success", "脚本位置已复制");
            }}>
              <IconCopy /> 复制位置
            </Button>
          ) : undefined
        }
      />
      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[280px_minmax(0,1fr)]">
        <Card title="文件" flush className="lg:sticky lg:top-0">
          <div className="p-3">
            <SearchInput value={q} onChange={setQ} placeholder="搜索脚本文件名…" />
          </div>
          <div className="max-h-[70vh] overflow-y-auto pb-2">
            {shown.length === 0 ? (
              <div className="px-4 py-6 text-center text-xs font-medium text-quaternary">没有匹配的文件</div>
            ) : shown.map((f) => (
              <button key={f.name} onClick={() => open(f.name, 0)}
                className={`flex w-full cursor-pointer items-center gap-2 px-4 py-2 text-left transition duration-100 ease-linear ${
                  f.name === name ? "bg-primary_hover" : "hover:bg-primary_hover"
                }`}>
                <span className={`min-w-0 flex-1 truncate font-mono text-xs ${
                  f.name === name ? "text-primary" : "text-tertiary"
                }`}>{f.name}</span>
                <span className="shrink-0 text-[11px] tabular-nums text-quaternary">{f.lines}</span>
              </button>
            ))}
          </div>
        </Card>
        <Card flush className="overflow-hidden">
          {err ? (
            <EmptyState title={`读取失败：${err}`} />
          ) : !content ? (
            <Loading text={loading ? "读取脚本…" : "选择左侧脚本"} />
          ) : (
            <div ref={codeRef} className="max-h-[72vh] overflow-auto">
              <pre className="min-w-full font-mono text-xs leading-5">
                {lines.map((l, i) => (
                  <div key={i} data-line={i + 1}
                    className={`flex ${i + 1 === target ? "bg-brand-secondary/15" : ""}`}>
                    <span className={`w-14 shrink-0 select-none px-2 text-right ${
                      i + 1 === target ? "text-fg-brand-primary" : "text-quaternary"
                    }`}>{i + 1}</span>
                    <span className="whitespace-pre-wrap break-words pr-4 text-secondary">{l || " "}</span>
                  </div>
                ))}
              </pre>
            </div>
          )}
        </Card>
      </div>
    </>
  );
}
