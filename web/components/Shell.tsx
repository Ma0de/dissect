"use client";
import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  fetchModel, getState, getTask, deleteGame, saveAsDownload, uploadFiles, relPath,
  pickNative, unpackPath, revealInExplorer, filesFromDrop, cancelTask, startWebBuild,
  GameModel, TaskState, AppState,
} from "../lib/api";
import {
  Button, IconButton, MenuPanel, MenuItem, MenuSeparator,
  ContextMenuProvider, Breadcrumb, Modal,
} from "./ui";
import {
  IconHome, IconChat, IconChoice, IconFlow, IconDB, IconLayers, IconCode,
  IconFolderOpen, IconFileZip, IconReveal, IconTrash, IconDownload, IconRefresh,
  IconMoon, IconSun, IconX, IconAlert,
} from "./icons";

type MenuKey = "file" | "edit" | "help" | null;
type ToastType = "error" | "success";
interface Toast { id: number; type: ToastType; msg: string; }

interface Ctx {
  model: GameModel | null;
  modelErr: string | null;
  state: AppState | null;
  task: TaskState | null;
  busy: boolean;
  uploadPct: number | null;
  stopping: boolean;
  offline: string | null;
  refreshModel: () => Promise<void>;
  openPath: (path: string) => Promise<void>;
  stopTask: () => void;
  buildWeb: (allowWindows: boolean) => Promise<void>;
  addToast: (type: ToastType, msg: string) => void;
}

const ModelCtx = createContext<Ctx>({
  model: null, modelErr: null, state: null, task: null, busy: false, uploadPct: null,
  stopping: false, offline: null, refreshModel: async () => {}, openPath: async () => {},
  stopTask: () => {}, buildWeb: async () => {}, addToast: () => {},
});
export const useModel = () => useContext(ModelCtx);

const PAGE_TITLES: Record<string, string> = {
  "/": "总览", "/story": "剧情文案", "/choices": "选项分支", "/flow": "流程图",
  "/fields": "字段", "/resources": "资源", "/headers": "头文件",
};

export default function Shell({ children }: { children: React.ReactNode }) {
  const [model, setModel] = useState<GameModel | null>(null);
  const [modelErr, setModelErr] = useState<string | null>(null);
  const [state, setState] = useState<AppState | null>(null);
  const [task, setTask] = useState<TaskState | null>(null);
  const [menu, setMenu] = useState<MenuKey>(null);
  const [versionDlg, setVersionDlg] = useState(false);
  const [dragDepth, setDragDepth] = useState(0);
  const [uploadPct, setUploadPct] = useState<number | null>(null);
  const [picking, setPicking] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [stopping, setStopping] = useState(false);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [dark, setDark] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [offline, setOffline] = useState<string | null>(null);
  const menubarRef = useRef<HTMLDivElement>(null);
  const pollFailsRef = useRef(0);
  const router = useRouter();
  const pathname = usePathname();

  const busy = (!!task && !task.done) || uploadPct !== null || deleting;
  const mode = state?.mode ?? "workspace";
  const hasModel = !!model;

  useEffect(() => {
    setDark(document.documentElement.classList.contains("dark-mode"));
  }, []);
  const toggleDark = useCallback(() => {
    setDark((d) => {
      const next = !d;
      document.documentElement.classList.toggle("dark-mode", next);
      try { localStorage.setItem("dissect-theme", next ? "dark" : "light"); } catch {}
      return next;
    });
  }, []);

  const addToast = useCallback((type: ToastType, msg: string) => {
    const id = Date.now() + Math.random();
    setToasts((ts) => [...ts.slice(-3), { id, type, msg }]);
    setTimeout(() => setToasts((ts) => ts.filter((t) => t.id !== id)),
      type === "error" ? 8000 : 4000);
  }, []);
  const dismissToast = useCallback((id: number) => {
    setToasts((ts) => ts.filter((t) => t.id !== id));
  }, []);

  const refreshModel = useCallback(async () => {
    try {
      setModel(await fetchModel());
      setModelErr(null);
    } catch (e: any) {
      setModel(null);
      setModelErr(e?.message || String(e));
    }
  }, []);

  useEffect(() => {
    refreshModel();
    getState().then((s) => {
      setState(s);
      setOffline(null);
      if (s.task && s.task.id && !s.task.done) setTask(s.task);
    }).catch(() => {
      setOffline("与 dissect 服务的连接已断开，请重新启动 dissect");
    });
  }, [refreshModel]);

  const toastRef = useRef(addToast);
  toastRef.current = addToast;
  const errTaskRef = useRef<string | null>(null);
  useEffect(() => {
    if (!task || task.done) return;
    const t = setInterval(async () => {
      const cur = await getTask().catch(() => null);
      if (!cur) {
        pollFailsRef.current += 1;
        if (pollFailsRef.current >= 2) setOffline("与 dissect 服务的连接已断开，正在重试…");
        return;
      }
      pollFailsRef.current = 0;
      setOffline(null);
      if (!cur.id) {
        setStopping(false);
        setTask(null);
        toastRef.current("error", "任务已中断：与服务端的会话已丢失");
        return;
      }
      setTask(cur);
      if (cur.done && cur.ok) {
        if (cur.kind === "web") {
          getState().then(setState).catch(() => {});
          setStopping(false);
          toastRef.current("success", "Web 版已生成");
          setTimeout(() => setTask((t) => (t && t.id === cur.id ? null : t)), 1500);
          return;
        }
        await refreshModel();
        router.push("/");
        setStopping(false);
        toastRef.current("success", `解包完成${cur.game ? `：${cur.game}` : ""}`);
        setTimeout(() => setTask((t) => (t && t.id === cur.id ? null : t)), 4000);
      } else if (cur.done && !cur.ok) {
        setStopping(false);
        if (cur.error === "任务已取消") {
          toastRef.current("success", "已暂停并结束任务");
          setTimeout(() => setTask((t) => (t && t.id === cur.id ? null : t)), 700);
          return;
        }
        if (errTaskRef.current !== cur.id) {
          errTaskRef.current = cur.id;
          toastRef.current("error", cur.error || "解包失败");
        }
      }
    }, 500);
    return () => clearInterval(t);
  }, [task?.done, task?.id, refreshModel, router]);

  useEffect(() => {
    let alive = true;
    const ping = async () => {
      try {
        await getState();
        if (alive) setOffline(null);
      } catch {
        if (alive) setOffline("与 dissect 服务的连接已断开，请重新启动 dissect");
      }
    };
    const t = setInterval(ping, 4000);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, []);

  useEffect(() => {
    document.title = model ? `${model.gameName} - dissect` : "dissect";
  }, [model]);

  const startUnpack = useCallback(async (files: File[]) => {
    if (!files.length || busy) return;
    if (mode !== "workspace") {
      addToast("error", "浏览模式（serve <目录>）不能上传；请直接双击 dissect 启动");
      return;
    }
    setTask(null);
    setUploadPct(0);
    router.push("/");
    try {
      const { id } = await uploadFiles(
        files.map((f) => ({ file: f, path: relPath(f) })),
        (p) => setUploadPct(p),
      );
      setUploadPct(null);
      setStopping(false);
      setTask({ id, stage: "准备解包", pct: 0, done: false, ok: false });
    } catch (e: any) {
      setUploadPct(null);
      addToast("error", e?.message || String(e));
    }
  }, [busy, mode, addToast, router]);

  const pickAndUnpack = useCallback(async (kind: "dir" | "zip") => {
    if (picking || busy) return;
    setPicking(true);
    try {
      const { path, error } = await pickNative(kind);
      if (error) { addToast("error", error); return; }
      if (!path) return;
      const { id } = await unpackPath(path);
      setStopping(false);
      setTask({ id, stage: "准备解包", pct: 0, done: false, ok: false });
      router.push("/");
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    } finally {
      setPicking(false);
    }
  }, [picking, busy, addToast, router]);

  const openPath = useCallback(async (path: string) => {
    if (busy) return;
    try {
      const { id } = await unpackPath(path);
      setStopping(false);
      setTask({ id, stage: "准备解包", pct: 0, done: false, ok: false });
      router.push("/");
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    }
  }, [busy, addToast, router]);

  const stopTask = useCallback(async () => {
    if (!task) return;
    if (task.done) {
      setTask(null);
      setStopping(false);
      return;
    }
    setStopping(true);
    try {
      await cancelTask();
    } catch (e: any) {
      setStopping(false);
      addToast("error", e?.message || String(e));
    }
  }, [task, addToast]);

  const buildWeb = useCallback(async (allowWindows: boolean) => {
    try {
      const { id } = await startWebBuild(allowWindows);
      setStopping(false);
      setTask({ id, kind: "web", stage: "web-check", pct: 0, done: false, ok: false });
      router.push("/");
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    }
  }, [addToast, router]);

  const doDelete = useCallback(async () => {
    const name = state?.game || model?.gameName || "";
    if (!confirm(`删除${name ? `「${name}」` : "当前"}的解包结果？此操作不可恢复。`)) return;
    setDeleting(true);
    try {
      await deleteGame();
      setTask(null);
      setModel(null);
      await refreshModel();
      getState().then(setState).catch(() => {});
      router.push("/");
      addToast("success", "已删除解包结果");
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    } finally {
      setDeleting(false);
    }
  }, [state, model, refreshModel, router, addToast]);

  const doSaveAs = useCallback(async () => {
    try {
      await saveAsDownload();
      addToast("success", "结果包已开始下载");
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    }
  }, [addToast]);

  const doReveal = useCallback(async () => {
    try {
      await revealInExplorer();
      addToast("success", "已在文件管理器中定位输出目录");
    } catch (e: any) {
      addToast("error", e?.message || String(e));
    }
  }, [addToast]);

  useEffect(() => {
    let depth = 0;
    const hasFiles = (e: DragEvent) =>
      Array.from(e.dataTransfer?.types || []).includes("Files");
    const onDragEnter = (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      depth++;
      setDragDepth(depth);
    };
    const onDragOver = (e: DragEvent) => {
      if (hasFiles(e)) e.preventDefault();
    };
    const onDragLeave = () => {
      depth = Math.max(0, depth - 1);
      setDragDepth(depth);
    };
    const onDrop = async (e: DragEvent) => {
      e.preventDefault();
      depth = 0;
      setDragDepth(0);
      if (e.dataTransfer) startUnpack(await filesFromDrop(e.dataTransfer));
    };
    window.addEventListener("dragenter", onDragEnter);
    window.addEventListener("dragover", onDragOver);
    window.addEventListener("dragleave", onDragLeave);
    window.addEventListener("drop", onDrop);
    return () => {
      window.removeEventListener("dragenter", onDragEnter);
      window.removeEventListener("dragover", onDragOver);
      window.removeEventListener("dragleave", onDragLeave);
      window.removeEventListener("drop", onDrop);
    };
  }, [startUnpack]);

  useEffect(() => {
    if (menu === null) return;
    const onDown = (e: MouseEvent) => {
      if (menubarRef.current && !menubarRef.current.contains(e.target as Node)) setMenu(null);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenu(null);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [menu]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "o") {
        e.preventDefault();
        if (mode === "workspace" && !busy) pickAndUnpack("dir");
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [mode, busy, pickAndUnpack]);

  const canUpload = mode === "workspace" && !busy && !picking;
  const path = pathname.length > 1 && pathname.endsWith("/") ? pathname.slice(0, -1) : pathname;

  const nav = [
    { href: "/", label: "总览", icon: <IconHome />, count: "" },
    { href: "/story", label: "剧情文案", icon: <IconChat />, count: model ? String(model.dialogues.length) : "" },
    { href: "/choices", label: "选项分支", icon: <IconChoice />, count: model ? String(model.choices.length) : "" },
    { href: "/flow", label: "流程图", icon: <IconFlow />, count: model ? String(model.flow.nodes.length) : "" },
    { href: "/fields", label: "字段", icon: <IconDB />, count: model ? String(model.fields.length) : "" },
    { href: "/resources", label: "资源", icon: <IconLayers />, count: model ? String(model.assets.length) : "" },
    { href: "/headers", label: "头文件", icon: <IconCode />, count: model && model.headers.length ? String(model.headers.length) : "" },
  ];

  const menuBtnCls = (open: boolean) =>
    `inline-flex cursor-pointer items-center rounded-md px-2 py-1 text-sm font-semibold outline-focus-ring transition duration-100 ease-linear select-none ${
      open ? "bg-primary_hover text-primary" : "text-secondary hover:bg-primary_hover hover:text-primary"
    }`;

  const crumbItems = [
    { label: "home", onClick: () => router.push("/") },
    {
      label:
        uploadPct !== null || (task && (!task.done || !task.ok || !model))
          ? (task?.kind === "web" ? "Web 版" : "解包")
          : !model
            ? "打开游戏文件夹"
            : PAGE_TITLES[path] || "总览",
    },
  ];

  return (
    <ModelCtx.Provider value={{
      model, modelErr, state, task, busy, uploadPct, stopping, offline, refreshModel,
      openPath, stopTask, buildWeb, addToast,
    }}>
      <ContextMenuProvider>
        <div className="flex h-screen flex-col overflow-hidden">
          <header className="relative z-30 flex h-10 shrink-0 items-center justify-between gap-3 border-b border-secondary bg-primary px-4">
            <div className="flex min-w-0 items-center gap-1">
              <button className="mr-1 inline-flex size-7 cursor-pointer items-center justify-center rounded-md text-fg-quaternary transition hover:bg-primary_hover hover:text-secondary lg:hidden"
                aria-label="菜单" onClick={() => setSidebarOpen(true)}>
                <svg viewBox="0 0 20 20" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                  <path d="M3 5.5h14M3 10h14M3 14.5h14" />
                </svg>
              </button>
              <div ref={menubarRef} className="flex items-center gap-1">
                <div className="relative">
                  <button className={menuBtnCls(menu === "file")} onClick={() => setMenu(menu === "file" ? null : "file")}>文件</button>
                  {menu === "file" && (
                    <div className="absolute left-0 top-full mt-1">
                      <MenuPanel>
                        <MenuItem item={{ label: "打开文件夹…", icon: <IconFolderOpen />, shortcut: "Ctrl+O", disabled: !canUpload, onClick: () => { setMenu(null); pickAndUnpack("dir"); } }} />
                        <MenuItem item={{ label: "打开 zip / .rpa…", icon: <IconFileZip />, disabled: !canUpload, onClick: () => { setMenu(null); pickAndUnpack("zip"); } }} />
                        <MenuSeparator />
                        <MenuItem item={{ label: "在资源管理器中显示", icon: <IconReveal />, disabled: !hasModel || busy, onClick: () => { setMenu(null); doReveal(); } }} />
                        <MenuItem item={{ label: "解包并保存为…", icon: <IconDownload />, disabled: !hasModel || busy, onClick: () => { setMenu(null); doSaveAs(); } }} />
                      </MenuPanel>
                    </div>
                  )}
                </div>
                <div className="relative">
                  <button className={menuBtnCls(menu === "edit")} onClick={() => setMenu(menu === "edit" ? null : "edit")}>编辑</button>
                  {menu === "edit" && (
                    <div className="absolute left-0 top-full mt-1">
                      <MenuPanel>
                        <MenuItem item={{ label: "删除解包结果", icon: <IconTrash />, danger: true, disabled: !hasModel || busy || mode !== "workspace", onClick: () => { setMenu(null); doDelete(); } }} />
                      </MenuPanel>
                    </div>
                  )}
                </div>
                <div className="relative">
                  <button className={menuBtnCls(menu === "help")} onClick={() => setMenu(menu === "help" ? null : "help")}>帮助</button>
                  {menu === "help" && (
                    <div className="absolute left-0 top-full mt-1">
                      <MenuPanel>
                        <MenuItem item={{ label: `版本 v${state?.version || "0.5.0"}`, icon: <IconRefresh />, onClick: () => { setMenu(null); setVersionDlg(true); } }} />
                      </MenuPanel>
                    </div>
                  )}
                </div>
              </div>
            </div>

            <IconButton aria-label={dark ? "切换到亮色" : "切换到深色"}
              title={dark ? "切换到亮色" : "切换到深色"} onClick={toggleDark}>
              {dark ? <IconSun /> : <IconMoon />}
            </IconButton>
          </header>

          {offline && (
            <div className="flex h-9 shrink-0 items-center justify-center gap-2 border-b border-utility-red-200 bg-error-primary px-4 text-xs font-semibold text-utility-red-700">
              <IconAlert size={14} />
              <span className="truncate">{offline}</span>
            </div>
          )}

          <div className="flex min-h-0 flex-1">
            {sidebarOpen && (
              <div className="fixed inset-0 z-40 bg-alpha-black/40 lg:hidden" onClick={() => setSidebarOpen(false)} />
            )}
            <aside className={`${sidebarOpen ? "flex" : "hidden"} fixed inset-y-0 left-0 z-50 w-68 max-w-full flex-col overflow-y-auto border-r border-secondary bg-primary lg:static lg:z-auto lg:flex lg:shrink-0`}>
              <div className="flex items-center justify-between px-5 pb-1 pt-4">
                <span className="text-xs font-semibold text-quaternary">视图</span>
                <button className="lg:hidden" onClick={() => setSidebarOpen(false)}>
                  <IconX size={16} className="text-fg-quaternary" />
                </button>
              </div>
              <div className="px-3 pb-4">
                {nav.map((n) => {
                  const active = path === n.href;
                  const enabled = n.href === "/" || (hasModel && !busy);
                  if (!enabled) {
                    return (
                      <div key={n.href} title="请先打开游戏文件" aria-disabled="true"
                        className="group relative flex w-full cursor-not-allowed select-none items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-semibold text-quaternary opacity-40">
                        <span className="[&_svg]:size-4 [&_svg]:stroke-[2.25px] text-fg-quaternary">{n.icon}</span>
                        <span className="flex-1 truncate">{n.label}</span>
                        {n.count && <span className="text-xs font-medium text-quaternary">{n.count}</span>}
                      </div>
                    );
                  }
                  return (
                    <Link key={n.href} href={n.href} onClick={() => setSidebarOpen(false)}
                      className={`group relative flex w-full cursor-pointer select-none items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-semibold outline-focus-ring transition duration-100 ease-linear ${
                        active ? "bg-primary_hover text-primary" : "text-tertiary hover:bg-primary_hover hover:text-secondary"
                      }`}>
                      <span className={`[&_svg]:size-4 [&_svg]:stroke-[2.25px] ${active ? "text-fg-brand-primary" : "text-fg-quaternary"}`}>
                        {n.icon}
                      </span>
                      <span className="flex-1 truncate">{n.label}</span>
                      {n.count && (
                        <span className={`text-xs font-medium ${active ? "text-tertiary" : "text-quaternary"}`}>{n.count}</span>
                      )}
                    </Link>
                  );
                })}
              </div>
            </aside>

            <div className="flex min-w-0 flex-1 flex-col">
              <div className="flex h-11 shrink-0 items-center gap-2 border-b border-secondary bg-primary px-5">
                <Breadcrumb items={crumbItems} />
              </div>
              <main className="min-h-0 flex-1 overflow-y-auto px-6 py-6">
                <div key={pathname} className="animate-page-in mx-auto w-full">
                  {children}
                </div>
              </main>
            </div>
          </div>

          {dragDepth > 0 && (
            <div className="pointer-events-none fixed inset-0 z-[90] flex items-center justify-center bg-primary/80 text-md font-semibold text-fg-brand-primary">
              <div className="absolute inset-2.5 rounded-xl border-2 border-dashed border-brand-400" />
              松开以上传并解包
            </div>
          )}

          <div className="fixed bottom-6 right-4 z-[110] flex max-w-[min(420px,calc(100vw-32px))] flex-col gap-2">
            {toasts.map((t) => (
              <div key={t.id}
                className="flex items-start gap-2.5 rounded-lg bg-primary_alt px-3.5 py-2.5 shadow-lg ring-1 ring-secondary_alt animate-toast-in">
                <span className={`mt-1.5 size-2 shrink-0 rounded-full ${t.type === "success" ? "bg-fg-success-primary" : "bg-fg-error-primary"}`} />
                <span className="flex-1 break-all text-sm font-medium text-secondary">{t.msg}</span>
                <button className="cursor-pointer text-quaternary hover:text-secondary" onClick={() => dismissToast(t.id)}>
                  <IconX size={14} />
                </button>
              </div>
            ))}
          </div>

          {versionDlg && (
            <Modal title={`dissect v${state?.version || "0.5.0"}`}
              sub={`${mode === "workspace" ? "WebUI 工作区模式" : "浏览模式"}`}
              onClose={() => setVersionDlg(false)}
              actions={<Button onClick={() => setVersionDlg(false)}>关闭</Button>} />
          )}
        </div>
      </ContextMenuProvider>
    </ModelCtx.Provider>
  );
}
