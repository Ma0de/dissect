"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useModel } from "../components/Shell";
import StartView from "../components/StartView";
import TaskView from "../components/TaskView";
import { Badge, Button, IconButton, Alert, Modal, Spinner, ProgressBar, useContextMenu } from "../components/ui";
import {
  IconChat, IconChoice, IconFlow, IconDB, IconDoc, IconReveal, IconCopy, IconAudio,
} from "../components/icons";
import { humanSize, exportURL, revealInExplorer, checkWeb, pickNative, exportStaticSite, WebExportResult } from "../lib/api";

const KIND_BAR: Record<string, string> = {
  image: "bg-brand-500",
  audio: "bg-green-400",
  script: "bg-blue-400",
  font: "bg-yellow-400",
  video: "bg-red-400",
  other: "bg-neutral-300",
};

export default function Home() {
  const { model, task, uploadPct, state, addToast, buildWeb } = useModel();
  const openCtx = useContextMenu();
  const [conflictImports, setConflictImports] = useState<string[] | null>(null);
  const [checkingWeb, setCheckingWeb] = useState(false);
  const [exportDlg, setExportDlg] = useState(false);
  const [exportPhase, setExportPhase] = useState<"idle" | "picking" | "exporting">("idle");
  const [exportProgress, setExportProgress] = useState<{ done: number; total: number } | null>(null);
  const [exportElapsed, setExportElapsed] = useState(0);
  const [exported, setExported] = useState<WebExportResult | null>(null);
  const [revealing, setRevealing] = useState(false);
  const web = state?.web;
  const webBusy = !!task && !task.done && task.kind === "web";

  useEffect(() => {
    if (exportPhase === "idle") {
      setExportElapsed(0);
      return;
    }
    const t = setInterval(() => setExportElapsed((v) => v + 1), 1000);
    return () => clearInterval(t);
  }, [exportPhase]);

  if (uploadPct !== null || (task && (!task.done || !task.ok || !model))) {
    return <TaskView />;
  }
  if (!model) {
    return <StartView />;
  }

  const onBuildWeb = async () => {
    if (checkingWeb || webBusy) return;
    setCheckingWeb(true);
    try {
      const c = await checkWeb();
      if (!c.supported) {
        addToast("error", c.reason || "该游戏暂不支持生成 Web 版");
        return;
      }
      if (c.windowsImports.length > 0) {
        setConflictImports(c.windowsImports);
        return;
      }
      await buildWeb(false);
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    } finally {
      setCheckingWeb(false);
    }
  };

  const onExportStatic = async (format: "folder" | "zip") => {
    if (exportPhase !== "idle") return;
    setExportPhase("picking");
    addToast("info", "正在打开系统文件选择框，请选择导出位置");
    try {
      const { path, error } = await pickNative("export-dir");
      if (error) {
        addToast("error", error);
        return;
      }
      if (!path) {
        addToast("info", "已取消导出");
        return;
      }
      setExportPhase("exporting");
      setExportProgress({ done: 0, total: web?.bytes || 0 });
      const res = await exportStaticSite(path, format, (p) => setExportProgress(p));
      setExported(res);
      addToast("success", "静态站点已导出");
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    } finally {
      setExportPhase("idle");
      setExportProgress(null);
    }
  };

  const onReveal = async (path?: string) => {
    if (revealing) return;
    setRevealing(true);
    try {
      await revealInExplorer(path);
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    } finally {
      setRevealing(false);
    }
  };

  const s = model.stats;
  const kinds = Object.entries(s.assetKinds || {}).sort((a, b) => b[1] - a[1]);
  const kindBytes = s.assetBytes || {};
  const assetTotal = model.assets.length || 1;
  const topChars = model.characters.filter((c) => c.lineCount > 0).slice(0, 10);
  const maxLines = topChars.length ? topChars[0].lineCount : 1;

  return (
    <>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-display-xs font-semibold tracking-[-0.36px] text-primary">
            {model.gameName || "未命名游戏"}
          </h1>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-tertiary">
            {model.engine && <Badge color="brand">{model.engine}</Badge>}
            <span>{model.engineVersion || "-"}</span>
            <span className="text-quaternary">·</span>
            <span className="font-mono text-xs">{model.gameId || "-"}</span>
            <span className="text-quaternary">·</span>
            <span>{model.assets.length} 个资产文件</span>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="secondary" size="sm" onClick={() => {
            navigator.clipboard?.writeText(state?.dataDir || "").catch(() => {});
            addToast("success", "输出目录路径已复制");
          }}>
            <IconCopy /> 复制输出路径
          </Button>
          {state?.dataDir && (
            <IconButton aria-label="在资源管理器中显示" title="在资源管理器中显示输出目录"
              disabled={revealing} onClick={() => onReveal()}>
              {revealing ? <Spinner size={16} /> : <IconReveal />}
            </IconButton>
          )}
        </div>
      </div>

      {model.notes.length > 0 && (
        <div className="mt-5">
          <Alert type="warning" title="解包过程备注">
            <div className="grid gap-x-10 gap-y-0.5 md:grid-cols-2 xl:grid-cols-3">
              {model.notes.map((n: string, i: number) => (
                <div key={i} className="flex items-start gap-2 py-0.5">
                  <IconDoc size={14} className="mt-1 shrink-0" />
                  <span>{n}</span>
                </div>
              ))}
            </div>
          </Alert>
        </div>
      )}

      <div className="mt-6 grid grid-cols-2 divide-x divide-secondary border-y border-secondary bg-secondary_alt md:grid-cols-5">
        <Stat label="剧情台词" value={s.dialogueCount} icon={<IconChat />}
          hint={`${model.characters.filter(c => c.lineCount > 0).length} 个角色发声`} />
        <Stat label="选项分支" value={s.choiceCount} icon={<IconChoice />}
          hint={`${model.flow.edges.filter(e => e.kind === "choice").length} 条分支边`} />
        <Stat label="流程节点" value={model.flow.nodes.length} icon={<IconFlow />}
          hint={`${model.flow.edges.length} 条跳转/调用边`} />
        <Stat label="字段" value={s.fieldCount} icon={<IconDB />}
          hint="store / persistent / 类字段" />
        <Stat label="语音映射" value={s.voiceMapped || 0} icon={<IconAudio />}
          hint="句台词已匹配语音" />
      </div>

      <section className="border-b border-secondary py-6">
        <SectionHead title="Game for Web" extra={
          web?.ready ? <Badge color="success">已生成</Badge> : <Badge color="gray">未生成</Badge>
        } />
        <div className="flex flex-wrap items-start justify-between gap-6">
          <div className="max-w-[720px] text-sm leading-6 text-tertiary">
            把当前 Ren'Py 游戏转换成可以直接在浏览器里运行的 Web 版（WebAssembly），由本机托管，点开就能玩。
            生成后也可以导出成独立的静态文件（目录或 ZIP），不依赖 dissect。
            仅支持 Ren'Py 8.x；使用 Live2D 的游戏暂不支持。首次生成需要联网下载对应版本的 Ren'Py 运行时，之后复用缓存。
          </div>
          <div className="flex shrink-0 items-center gap-3">
            {web?.ready && (
              <Button variant="secondary" size="sm" onClick={() => { setExported(null); setExportDlg(true); }}>
                导出静态站点
              </Button>
            )}
            {web?.ready && (
              <Button variant="secondary" size="sm" onClick={() => window.open("/play/", "_blank")}>
                试玩
              </Button>
            )}
            <Button size="sm" onClick={onBuildWeb} disabled={checkingWeb || webBusy}>
              {webBusy ? "生成中…" : web?.ready ? "重新生成" : "生成 Web 版"}
            </Button>
          </div>
        </div>
        {web?.ready && (
          <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-quaternary">
            <span>Ren'Py {web.version}</span>
            <span>·</span>
            <span>{humanSize(web.bytes || 0)}</span>
            {web.builtAt && (
              <>
                <span>·</span>
                <span>{new Date(web.builtAt).toLocaleString()}</span>
              </>
            )}
          </div>
        )}
      </section>

      <div className="grid grid-cols-1 lg:grid-cols-12">
        <section className="border-b border-secondary py-6 lg:col-span-8 lg:pr-10">
          <SectionHead title="资产构成" extra={<span className="text-xs font-medium text-quaternary">{model.assets.length} 个文件</span>} />
          <div className="flex h-2.5 w-full overflow-hidden rounded-full bg-tertiary">
            {kinds.map(([k, n]) => (
              <div key={k} className={KIND_BAR[k] || KIND_BAR.other}
                style={{ width: `${(n / assetTotal) * 100}%` }} title={`${k} · ${n}`} />
            ))}
          </div>
          <div className="mt-5 grid grid-cols-1 gap-x-10 gap-y-2.5 sm:grid-cols-2">
            {kinds.map(([k, n]) => (
              <div key={k} className="flex items-center gap-2.5 text-sm">
                <span className={`size-2.5 shrink-0 rounded-sm ${KIND_BAR[k] || KIND_BAR.other}`} />
                <span className="font-medium text-secondary">{k}</span>
                <span className="ml-auto tabular-nums text-tertiary">{n} 个</span>
                <span className="w-24 text-right tabular-nums text-quaternary">{humanSize(kindBytes[k] || 0)}</span>
                <span className="w-12 text-right tabular-nums text-quaternary">{((n / assetTotal) * 100).toFixed(1)}%</span>
              </div>
            ))}
          </div>
        </section>

        <section className="border-b border-secondary py-6 lg:col-span-4 lg:border-l lg:pl-10">
          <SectionHead title="角色台词 TOP" extra={
            <Link className="text-sm font-semibold text-fg-brand-secondary hover:text-fg-brand-primary" href="/story">全部 →</Link>
          } />
          <div className="grid grid-cols-1 gap-x-8 sm:grid-cols-2 lg:grid-cols-1">
            {topChars.map((c) => (
              <div key={c.tag} className="flex items-center gap-2.5 py-1.5 text-sm">
                <span className="size-2 shrink-0 rounded-full" style={{ background: c.color || "#b692f6" }} />
                <span className="font-medium text-primary">{c.name}</span>
                <span className="truncate font-mono text-xs text-quaternary">{c.tag}</span>
                <div className="ml-auto flex items-center gap-3">
                  <div className="hidden h-1.5 w-20 rounded-full bg-tertiary sm:block">
                    <div className="h-full rounded-full bg-brand-500"
                      style={{ width: `${(c.lineCount / maxLines) * 100}%` }} />
                  </div>
                  <span className="w-11 text-right tabular-nums text-xs font-medium text-quaternary">{c.lineCount}</span>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="border-b border-secondary py-6 lg:col-span-8 lg:pr-10">
          <SectionHead title="导出交付物" />
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {[
              ["dialogues.csv", "台词 CSV"], ["dialogues_by_character.txt", "按角色分组文案"],
              ["choices.csv", "选项 CSV"], ["dialogues.json", "台词 JSON"],
              ["fields.csv", "字段 CSV"], ["fields.json", "字段 JSON"],
              ["flow.mmd", "流程图 Mermaid"], ["flow.json", "流程图 JSON"],
              ["Model.h", "Model.h 头文件"],
            ].map(([file, label]) => (
              <a key={file} href={exportURL(file)}
                className="flex items-center justify-between gap-2 rounded-lg bg-primary px-3 py-2 text-sm font-semibold text-secondary shadow-xs-skeuomorphic ring-1 ring-primary ring-inset transition duration-100 ease-linear hover:bg-primary_hover">
                <span className="truncate">{label}</span>
                <span className="truncate font-mono text-[11px] font-medium text-quaternary">{file}</span>
              </a>
            ))}
          </div>
        </section>

        <section className="border-b border-secondary py-6 lg:col-span-4 lg:border-l lg:pl-10">
          <SectionHead title="引擎信息" />
          <div className="grid grid-cols-1 gap-x-8 gap-y-2.5 text-sm sm:grid-cols-2 lg:grid-cols-1">
            <Row k="引擎" v={<Badge color="brand">{model.engine}</Badge>} />
            <Row k="版本" v={model.engineVersion || "-"} />
            <Row k="游戏代号" v={<span className="font-mono text-xs">{model.gameId}</span>} />
            <Row k="脚本文件" v={`${model.scriptFiles.length} 个（${model.scriptFiles.filter(f => f.decompiled).length} 已反编译）`} />
            <div className="sm:col-span-2 lg:col-span-1">
              <Row k="分析输入" v={
                state?.dataDir ? (
                  <button
                    className="max-w-full break-all text-right font-mono text-xs text-tertiary underline decoration-neutral-300 underline-offset-3 hover:text-secondary"
                    onContextMenu={(e) => openCtx(e, [{ label: "复制路径", icon: <IconCopy />, onClick: () => { navigator.clipboard?.writeText(state.dataDir || "").catch(() => {}); addToast("success", "路径已复制"); } }])}
                    title="右键复制路径">
                    {model.sourceDir || "-"}
                  </button>
                ) : (
                  <span className="break-all font-mono text-xs text-quaternary">{model.sourceDir || "-"}</span>
                )
              } />
            </div>
          </div>
        </section>
      </div>

      {exportDlg && (
        <Modal
          title={
            exported ? "静态站点已导出"
              : exportPhase === "picking" ? "选择导出位置"
                : exportPhase === "exporting" ? "正在导出静态站点"
                  : "导出静态站点"
          }
          sub={
            exported ? undefined
              : exportPhase === "picking" ? "已打开系统文件选择框，请在弹出的窗口里选择位置"
                : exportPhase === "exporting" ? "正在写入文件，请保持本页面打开"
                  : "导出的文件不依赖 dissect，也不需要后端"
          }
          onClose={() => { if (exportPhase !== "idle") return; setExportDlg(false); setExported(null); }}
          actions={
            exported ? (
              <>
                <Button variant="secondary" onClick={() => {
                  navigator.clipboard?.writeText(exported.path).catch(() => {});
                  addToast("success", "路径已复制");
                }}>复制路径</Button>
                <Button variant="secondary" disabled={revealing} onClick={() => onReveal(exported.path)}>
                  {revealing ? (<><Spinner size={16} /> 打开中…</>) : "在资源管理器中显示"}
                </Button>
                <Button onClick={() => { setExportDlg(false); setExported(null); }}>完成</Button>
              </>
            ) : exportPhase !== "idle" ? (
              <Button variant="secondary" disabled>
                <Spinner size={16} /> {exportPhase === "picking" ? "等待选择位置…" : "导出中…"}
              </Button>
            ) : (
              <>
                <Button variant="secondary" onClick={() => setExportDlg(false)}>取消</Button>
                <Button variant="secondary" onClick={() => onExportStatic("folder")}>复制到文件夹</Button>
                <Button onClick={() => onExportStatic("zip")}>打包为 ZIP</Button>
              </>
            )
          }
        >
          {exported ? (
            <div className="mt-4 space-y-2 text-sm leading-6 text-tertiary">
              <p className="break-all font-mono text-xs text-primary">{exported.path}</p>
              <p>共 {exported.files} 个文件，约 {humanSize(exported.bytes)}。</p>
            </div>
          ) : exportPhase === "picking" ? (
            <div className="mt-4 flex items-start gap-3 rounded-lg bg-secondary_alt p-3.5">
              <Spinner className="mt-0.5" />
              <div className="text-sm leading-6 text-tertiary">
                <p className="font-medium text-secondary">系统文件选择框已打开，并且已置顶</p>
                <p className="mt-1">在窗口里选好文件夹后点「选择文件夹 / 打开」，导出会自动开始；点「取消」则放弃。</p>
                <p className="mt-1 text-xs text-quaternary">
                  已等待 {exportElapsed} 秒 · 如果没看到窗口，按 Alt+Tab 或在任务栏里找一下
                </p>
              </div>
            </div>
          ) : exportPhase === "exporting" ? (
            <div className="mt-4 space-y-2.5 rounded-lg bg-secondary_alt p-3.5">
              <div className="flex items-center gap-2 text-sm font-medium text-secondary">
                <Spinner size={16} />
                正在写入文件…
                <span className="ml-auto font-mono text-xs font-normal text-quaternary">{exportElapsed}s</span>
              </div>
              <ProgressBar pct={exportProgress && exportProgress.total > 0 ? exportProgress.done / exportProgress.total : 0} />
              <div className="flex justify-between font-mono text-xs text-quaternary">
                <span>{humanSize(exportProgress?.done || 0)}</span>
                <span>{exportProgress && exportProgress.total > 0 ? humanSize(exportProgress.total) : "准备中…"}</span>
              </div>
            </div>
          ) : (
            <div className="mt-4 space-y-2 text-sm leading-6 text-tertiary">
              <p>导出的是完整的 Web 版文件：包含运行所需的全部内容，不依赖 dissect，也不需要后端。</p>
              <ul className="list-disc space-y-1 pl-5">
                <li>目录：复制一份到选定文件夹；</li>
                <li>ZIP：打包成单个压缩包；</li>
                <li>两种方式内容一致。</li>
              </ul>
              <p className="text-xs text-quaternary">约 {humanSize(web?.bytes || 0)}，导出需要一点时间。</p>
            </div>
          )}
        </Modal>
      )}

      {conflictImports && (
        <Modal
          title="检测到仅 Windows 可用的导入"
          sub={conflictImports.join("、")}
          onClose={() => setConflictImports(null)}
          actions={
            <>
              <Button variant="secondary" onClick={() => setConflictImports(null)}>取消</Button>
              <Button onClick={() => { setConflictImports(null); buildWeb(true); }}>继续生成</Button>
            </>
          }
        >
          <div className="mt-4 space-y-2 text-sm leading-6 text-tertiary">
            <p>这些模块只存在于 Windows 桌面版 Ren'Py。Web 版会注入兼容桩让脚本先加载，但：</p>
            <ul className="list-disc space-y-1 pl-5">
              <li>依赖它们的系统功能会静默失效，例如 Windows TTS、外部进程调用、窗口与截图控制；</li>
              <li>如果游戏在初始化阶段就直接调用这些系统 API，Web 版可能启动失败，任务日志里会给出具体错误；</li>
              <li>生成失败不会影响已有的解包结果，可以随时重试。</li>
            </ul>
            <p>你可以先继续生成，看看实际能不能跑起来。</p>
          </div>
        </Modal>
      )}
    </>
  );
}

function Stat({ label, value, hint, icon }: {
  label: string; value: React.ReactNode; hint: string; icon: React.ReactNode;
}) {
  return (
    <div className="px-6 py-5 first:pl-0 last:pr-0">
      <div className="flex items-center gap-2 text-sm font-medium text-tertiary [&_svg]:size-4 [&_svg]:text-fg-quaternary">
        {icon}{label}
      </div>
      <div className="mt-1.5 text-3xl font-semibold tabular-nums text-primary">{value}</div>
      <div className="mt-0.5 text-xs font-medium text-quaternary">{hint}</div>
    </div>
  );
}

function SectionHead({ title, extra }: { title: string; extra?: React.ReactNode }) {
  return (
    <div className="mb-4 flex items-center justify-between gap-3">
      <h2 className="text-md font-semibold text-primary">{title}</h2>
      {extra}
    </div>
  );
}

function Row({ k, v }: { k: string; v: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <span className="shrink-0 text-tertiary">{k}</span>
      <span className="min-w-0 break-all text-right text-primary">{v}</span>
    </div>
  );
}
