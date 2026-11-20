"use client";
import { useState } from "react";
import Link from "next/link";
import { useModel } from "../components/Shell";
import StartView from "../components/StartView";
import TaskView from "../components/TaskView";
import { Badge, Button, IconButton, Alert, Modal, useContextMenu } from "../components/ui";
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
  const [exporting, setExporting] = useState<"folder" | "zip" | null>(null);
  const [exported, setExported] = useState<WebExportResult | null>(null);
  const web = state?.web;
  const webBusy = !!task && !task.done && task.kind === "web";

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
    if (exporting) return;
    try {
      const { path, error } = await pickNative("export-dir");
      if (error) {
        addToast("error", error);
        return;
      }
      if (!path) return;
      setExporting(format);
      const res = await exportStaticSite(path, format);
      setExported(res);
      addToast("success", "静态站点已导出");
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    } finally {
      setExporting(null);
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
              onClick={() => { revealInExplorer().catch(() => {}); }}>
              <IconReveal />
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
            也可以导出成不依赖 dissect 的静态站点，上传到任意静态服务器对外提供。
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
                    onContextMenu={(e) => openCtx(e, [{ label: "复制路径", icon: <IconCopy />, onClick: () => navigator.clipboard?.writeText(state.dataDir || "") }])}
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
          title={exported ? "静态站点已导出" : "导出静态站点"}
          sub={exported ? undefined : "导出的站点不依赖 dissect，可直接部署到任意静态服务器"}
          onClose={() => { if (exporting) return; setExportDlg(false); setExported(null); }}
          actions={exported ? (
            <>
              <Button variant="secondary" onClick={() => {
                navigator.clipboard?.writeText(exported.path).catch(() => {});
                addToast("success", "路径已复制");
              }}>复制路径</Button>
              <Button variant="secondary" onClick={() => { revealInExplorer(exported.path).catch(() => {}); }}>
                在资源管理器中显示
              </Button>
              <Button onClick={() => { setExportDlg(false); setExported(null); }}>完成</Button>
            </>
          ) : (
            <>
              <Button variant="secondary" disabled={!!exporting} onClick={() => setExportDlg(false)}>取消</Button>
              <Button variant="secondary" disabled={!!exporting} onClick={() => onExportStatic("folder")}>
                {exporting === "folder" ? "导出中…" : "复制到文件夹"}
              </Button>
              <Button disabled={!!exporting} onClick={() => onExportStatic("zip")}>
                {exporting === "zip" ? "打包中…" : "打包为 ZIP"}
              </Button>
            </>
          )}
        >
          {exported ? (
            <div className="mt-4 space-y-2 text-sm leading-6 text-tertiary">
              <p className="break-all font-mono text-xs text-primary">{exported.path}</p>
              <p>共 {exported.files} 个文件，约 {humanSize(exported.bytes)}，目录里附带了一份 README.txt 部署说明。</p>
              <p>
                上传到任意静态 HTTP 服务器或静态托管平台即可；本机预览可在该目录执行{" "}
                <code className="rounded bg-tertiary px-1">python -m http.server</code>。
              </p>
            </div>
          ) : (
            <div className="mt-4 space-y-2 text-sm leading-6 text-tertiary">
              <p>导出的是完整静态站点：不依赖 dissect，也不需要任何后端，上传到静态服务器就能玩。</p>
              <ul className="list-disc space-y-1 pl-5">
                <li>目录：复制一份到选定文件夹，方便本机起服务器预览；</li>
                <li>ZIP：打包成单个压缩包，方便上传到托管平台；</li>
                <li>两种方式都会附带 README.txt（.wasm 的 MIME、体积与存档说明等）。</li>
              </ul>
              <p className="text-xs text-quaternary">站点约 {humanSize(web?.bytes || 0)}，导出需要一点时间。</p>
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
