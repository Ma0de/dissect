// 通用游戏模型 TS 类型（与 Go model.json 对应）+ 前端 API 封装
export interface Character {
  tag: string; name: string; color?: string; kind?: string;
  lineCount: number; sources?: string[];
}
export interface Field {
  name: string; scope: string; type: string; default?: string;
  keyword?: string; file?: string; line?: number; classOf?: string;
}
export interface Dialogue {
  id: number; file: string; line: number; speaker: string;
  text: string; plain?: string; cond?: string; label: string; withExpr?: string;
  voiceId?: string; audio?: string;
}
export interface Choice {
  id: number; file: string; line: number; label: string; text: string;
  cond?: string; target?: string; targetKind?: string;
}
export interface FlowNode { id: string; label: string; kind: string; file?: string; line?: number; }
export interface FlowEdge { from: string; to: string; kind: string; label?: string; cond?: string; file?: string; line?: number; }
export interface Asset {
  path: string; kind: string; size: number; usedAt?: string[]; imageName?: string;
}
export interface ScriptFile { path: string; lines: number; decompiled: boolean; }
export interface HeaderFile { name: string; engine: string; note?: string; path: string; }
export interface GameModel {
  gameName: string; gameId: string; engine: string; engineVersion: string;
  sourceDir?: string;
  stats: {
    labels: number; screens: number; dialogueCount: number; choiceCount: number;
    fieldCount: number; voiceMapped?: number;
    assetKinds?: Record<string, number>; assetBytes?: Record<string, number>;
  };
  characters: Character[]; fields: Field[]; dialogues: Dialogue[];
  choices: Choice[];
  flow: { nodes: FlowNode[]; edges: FlowEdge[] };
  assets: Asset[]; scriptFiles: ScriptFile[]; headers: HeaderFile[];
  notes: string[]; engineMeta?: Record<string, unknown>;
}

// ---- 服务端任务 / 应用状态 ----
export type TaskStepState = "pending" | "running" | "done" | "skipped" | "error";

export interface TaskStep {
  key: string;
  label: string;
  state: TaskStepState | string;
}

export interface TaskLog {
  time: string;
  level?: string;
  msg: string;
}

export interface TaskState {
  id: string; stage: string; pct: number; done: boolean; ok: boolean;
  kind?: string;
  error?: string; game?: string;
  engine?: string;
  current?: string;
  steps?: TaskStep[];
  logs?: TaskLog[];
}

export interface WebInfo {
  ready: boolean;
  version?: string;
  title?: string;
  bytes?: number;
  builtAt?: string;
}

export interface WebCheck {
  supported: boolean;
  reason?: string;
  engine: string;
  version: string;
  title: string;
  live2d: boolean;
  windowsImports: string[];
}

export interface AppState {
  version: string;
  mode: "workspace" | "browse";
  hasModel: boolean;
  game: string;
  dataDir?: string;
  task: TaskState;
  web?: WebInfo;
}

async function getJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const r = await fetch(url, { cache: "no-store", ...init });
  const j = await r.json().catch(() => null);
  if (!r.ok || (j && typeof j === "object" && (j as any).error)) {
    throw new Error((j && (j as any).error) || `请求失败 (${r.status})`);
  }
  return j as T;
}

// 归一化：把旧 model.json 里的 null 数组补成 []，杜绝 null.length 崩溃
const arr = <T,>(x: T[] | null | undefined): T[] => (Array.isArray(x) ? x : []);

export function normalizeModel(m: any): GameModel {
  m = m && typeof m === "object" ? m : {};
  const st = m.stats && typeof m.stats === "object" ? m.stats : {};
  return {
    ...m,
    stats: {
      labels: st.labels || 0, screens: st.screens || 0,
      dialogueCount: st.dialogueCount || 0, choiceCount: st.choiceCount || 0,
      fieldCount: st.fieldCount || 0, voiceMapped: st.voiceMapped || 0,
      assetKinds: st.assetKinds || {}, assetBytes: st.assetBytes || {},
    },
    characters: arr(m.characters),
    fields: arr(m.fields),
    dialogues: arr(m.dialogues),
    choices: arr(m.choices),
    flow: { nodes: arr(m.flow?.nodes), edges: arr(m.flow?.edges) },
    assets: arr(m.assets),
    scriptFiles: arr(m.scriptFiles),
    headers: arr(m.headers),
    notes: arr(m.notes),
  };
}

export async function fetchModel(): Promise<GameModel> {
  const j = await getJSON<any>("/api/model");
  return normalizeModel(j);
}

export function getState(): Promise<AppState> {
  return getJSON<AppState>("/api/state");
}

export function getTask(): Promise<TaskState> {
  return getJSON<TaskState>("/api/task");
}

export async function cancelTask(): Promise<void> {
  await getJSON("/api/task/cancel", { method: "POST" });
}

export function checkWeb(): Promise<WebCheck> {
  return getJSON<WebCheck>("/api/web/check", { method: "POST" });
}

export async function startWebBuild(allowWindows: boolean): Promise<{ id: string }> {
  const r = await fetch("/api/web/build", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ allowWindows }),
  });
  const j = await r.json().catch(() => null);
  if (!r.ok || !j || !j.id) throw new Error((j && j.error) || `请求失败 (${r.status})`);
  return j;
}

// 弹出服务端系统原生选择框（本地工具直接读本地路径，不走浏览器上传）
export async function pickNative(kind: "dir" | "zip" | "export-dir"): Promise<{ path: string; error?: string }> {
  const r = await fetch(`/api/pick?kind=${kind}`, { method: "POST" });
  const j = await r.json().catch(() => null);
  if (!j) throw new Error(`请求失败 (${r.status})`);
  return j;
}

export interface WebExportResult {
  path: string;
  bytes: number;
  files: number;
}

export interface WebExportProgress {
  done: number;
  total: number;
}

// 把已生成的 Web 版导出为不依赖 dissect 的静态站点（目录或 zip），边导出边回报进度
export async function exportStaticSite(
  dest: string,
  format: "folder" | "zip",
  onProgress?: (p: WebExportProgress) => void,
): Promise<WebExportResult> {
  const r = await fetch("/api/web/export", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ dest, format }),
  });
  if (!r.ok) {
    const j = await r.json().catch(() => null);
    throw new Error((j && j.error) || `导出失败 (${r.status})`);
  }
  if (!r.body) throw new Error("导出失败：当前浏览器不支持流式响应");
  const reader = r.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  let result: WebExportResult | null = null;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    let idx: number;
    while ((idx = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, idx).trim();
      buf = buf.slice(idx + 1);
      if (!line) continue;
      const ev = JSON.parse(line);
      if (ev.stage === "error") throw new Error(ev.error || "导出失败");
      if (ev.stage === "progress" && onProgress) {
        onProgress({ done: ev.done || 0, total: ev.total || 0 });
      }
      if (ev.stage === "done") result = ev as WebExportResult;
    }
  }
  if (!result) throw new Error("导出失败：没有收到完成事件");
  return result;
}

// 直接解包本地路径（文件 ▸ 打开 / 落地页按钮的主路径）
export async function unpackPath(path: string): Promise<{ id: string }> {
  const r = await fetch("/api/unpack-path", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path }),
  });
  const j = await r.json().catch(() => null);
  if (!r.ok || !j || !j.id) throw new Error((j && j.error) || `请求失败 (${r.status})`);
  return j;
}

export async function deleteGame(): Promise<void> {
  await getJSON("/api/delete", { method: "POST" });
}

// 在系统文件管理器中显示输出目录（Windows explorer /select、mac open -R、linux xdg-open）
export async function revealInExplorer(path?: string): Promise<void> {
  const q = path ? `?path=${encodeURIComponent(path)}` : "";
  await getJSON(`/api/reveal${q}`, { method: "POST" });
}

// 解包并保存为...：浏览器原生下载对话框即「保存为」
export async function saveAsDownload(): Promise<void> {
  const r = await fetch("/api/download", { method: "HEAD", cache: "no-store" });
  if (!r.ok) {
    const j = await fetch("/api/download", { cache: "no-store" }).then((x) => x.json()).catch(() => null);
    throw new Error((j && j.error) || "没有可保存的结果");
  }
  location.assign("/api/download");
}

// 上传并解包：XHR 以获得上传进度；path 为相对路径（保留目录结构）
export function uploadFiles(
  files: { file: File; path: string }[],
  onUploadPct: (pct: number) => void,
): Promise<{ id: string }> {
  return new Promise((resolve, reject) => {
    const fd = new FormData();
    for (const { file, path } of files) fd.append("files", file, path);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/unpack");
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onUploadPct(e.loaded / e.total);
    };
    xhr.onload = () => {
      let j: any = null;
      try { j = JSON.parse(xhr.responseText); } catch { /* ignore */ }
      if (xhr.status >= 200 && xhr.status < 300 && j && j.id) resolve(j);
      else reject(new Error((j && j.error) || `上传失败 (${xhr.status})`));
    };
    xhr.onerror = () => reject(new Error("网络错误：无法连接 dissect 服务"));
    xhr.send(fd);
  });
}

export function relPath(f: File): string {
  const anyF = f as any;
  const p = anyF.webkitRelativePath || anyF._rel || f.name;
  return p.replace(/^\/+/, "");
}

// ---- 拖拽：把拖入的文件/文件夹（含嵌套）展开成 File 列表 ----
export async function filesFromDrop(dt: DataTransfer): Promise<File[]> {
  const out: File[] = [];
  const items = dt.items;
  // webkitGetAsEntry 必须在事件处理同步阶段先取完
  const entries: any[] = [];
  if (items && items.length && typeof (items[0] as any).webkitGetAsEntry === "function") {
    for (let i = 0; i < items.length; i++) {
      const e = (items[i] as any).webkitGetAsEntry();
      if (e) entries.push(e);
    }
  }
  if (!entries.length) {
    return Array.from(dt.files || []);
  }
  for (const e of entries) await walkEntry(e, out);
  return out;
}

function walkEntry(entry: any, out: File[]): Promise<void> {
  if (!entry) return Promise.resolve();
  if (entry.isFile) {
    return new Promise((res) => {
      entry.file((f: File) => {
        (f as any)._rel = (entry.fullPath || f.name).replace(/^\/+/, "");
        out.push(f);
        res();
      }, () => res());
    });
  }
  if (entry.isDirectory) {
    const reader = entry.createReader();
    return new Promise((res) => {
      const batch = () => {
        reader.readEntries(async (list: any[]) => {
          if (!list.length) return res();
          for (const e of list) await walkEntry(e, out);
          batch(); // readEntries 单次最多返回 100 条，读到空为止
        }, () => res());
      };
      batch();
    });
  }
  return Promise.resolve();
}

export function humanSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

export function fileURL(path: string): string {
  return "/assets/" + path.split("/").map(encodeURIComponent).join("/");
}

export function exportURL(name: string): string {
  return "/api/export/" + encodeURIComponent(name);
}
