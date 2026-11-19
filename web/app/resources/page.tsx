// 资源页：类型过滤栏 + 树视图/列表/画廊 + 右键菜单（复制路径 / 定位）
"use client";
import { useMemo, useState } from "react";
import { useModel } from "../../components/Shell";
import {
  Card, Loading, EmptyState, SearchInput, Badge, Tabs, PageHead, ButtonGroup,
  kindBadgeColor, LoadMore, Tbl, Tr, Td, useContextMenu, TreeChevron,
} from "../../components/ui";
import {
  fileURL, humanSize, revealInExplorer,
} from "../../lib/api";
import {
  IconImage, IconCopy, IconReveal, IconFolder, IconFolderOpen,
  IconGrid, IconList, IconFile, IconAudio, IconFont, IconScript, IconVideo,
} from "../../components/icons";

type View = "tree" | "list";

export default function Assets() {
  const { model, modelErr: err, addToast } = useModel();
  const [kind, setKind] = useState("image");
  const [q, setQ] = useState("");
  const [view, setView] = useState<View>("tree");
  const [limit, setLimit] = useState(120);
  const openCtx = useContextMenu();

  const kinds = useMemo(() => {
    if (!model) return [];
    const m: Record<string, number> = {};
    for (const a of model.assets) m[a.kind] = (m[a.kind] || 0) + 1;
    return Object.entries(m).sort((x, y) => y[1] - x[1]);
  }, [model]);

  const assets = useMemo(() => {
    if (!model) return [];
    let as = model.assets.filter((a) => a.kind === kind);
    if (q.trim()) {
      const kw = q.trim().toLowerCase();
      as = as.filter((a) => a.path.toLowerCase().includes(kw) || (a.imageName || "").toLowerCase().includes(kw));
    }
    return as;
  }, [model, kind, q]);

  // 树视图：按目录分组
  const tree = useMemo(() => {
    const root: TreeNode = { name: "", path: "", dirs: new Map(), files: [] };
    for (const a of assets) {
      const parts = a.path.split("/");
      let cur = root;
      for (let i = 0; i < parts.length - 1; i++) {
        if (!cur.dirs.has(parts[i])) {
          cur.dirs.set(parts[i], {
            name: parts[i], path: cur.path ? `${cur.path}/${parts[i]}` : parts[i],
            dirs: new Map(), files: [],
          });
        }
        cur = cur.dirs.get(parts[i])!;
      }
      cur.files.push(a);
    }
    return root;
  }, [assets]);

  if (err) return <EmptyState title={`读取失败：${err}`} />;
  if (!model) return <Loading />;

  const copyPath = (p: string) => {
    navigator.clipboard?.writeText(p).catch(() => {});
    addToast("success", "路径已复制");
  };
  const ctxFor = (p: string) => [
    { label: "复制路径", icon: <IconCopy />, onClick: () => copyPath(p) },
    { label: "在资源管理器中显示", icon: <IconReveal />, onClick: () => revealInExplorer().catch((e) => addToast("error", String(e.message || e))) },
    { label: "在新标签页打开", icon: <IconFile />, onClick: () => window.open(fileURL(p), "_blank") },
  ];

  return (
    <>
      <PageHead
        title="资源"
        sub={`${model.assets.length} 个文件，来自 .rpa 档案与游戏目录`}
        actions={
          <ButtonGroup
            value={view}
            onChange={(v) => setView(v as View)}
            options={[
              { key: "tree", label: "树视图", icon: <IconList /> },
              { key: "list", label: kind === "image" ? "画廊" : "列表", icon: <IconGrid /> },
            ]}
          />
        }
      />
      <Tabs value={kind} onChange={(k) => { setKind(k); setLimit(120); }}
        tabs={kinds.map(([k, n]) => ({ key: k, label: k, count: n }))} />
      <div className="mb-4 mt-4 max-w-[480px]">
        <SearchInput value={q} onChange={setQ} placeholder="搜索文件路径 / image 名…" />
      </div>

      {assets.length === 0 && <EmptyState icon={<IconImage />} title="没有匹配的资源" />}

      {assets.length > 0 && view === "tree" && (
        <Card flush>
          <div className="p-2 font-mono text-sm">
            <TreeRow
              node={tree} depth={0} defaultOpen
              onCtx={(p) => ctxFor(p)} />
          </div>
        </Card>
      )}

      {assets.length > 0 && view === "list" && kind === "image" && (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-3">
          {assets.slice(0, limit).map((a) => (
            <a className="overflow-hidden rounded-lg bg-secondary_alt ring-1 ring-secondary transition duration-100 ease-linear hover:ring-primary"
              key={a.path} href={fileURL(a.path)} target="_blank"
              onContextMenu={(e) => { e.preventDefault(); openCtx(e, ctxFor(a.path)); }}>
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={fileURL(a.path)} alt={a.path} loading="lazy"
                className="block h-[120px] w-full object-cover"
                style={{ background: "repeating-conic-gradient(#f5f5f5 0 25%, #fafafa 0 50%) 50%/16px 16px" }} />
              <div className="truncate px-2.5 py-1.5 text-xs font-medium text-quaternary" title={a.path}>
                {a.imageName ? `${a.imageName} · ` : ""}{a.path.split("/").pop()}
              </div>
            </a>
          ))}
        </div>
      )}

      {assets.length > 0 && view === "list" && kind !== "image" && (
        <Card flush>
          <Tbl cols={["文件", "大小", ...(kind === "audio" ? ["试听"] : ["类型", "image 别名"])]}>
            {assets.slice(0, limit).map((a) => (
              <Tr key={a.path}>
                <Td mono className="max-w-[420px] truncate">
                  <button className="cursor-pointer hover:text-primary hover:underline"
                    onClick={() => window.open(fileURL(a.path), "_blank")}
                    onContextMenu={(e) => { e.preventDefault(); openCtx(e, ctxFor(a.path)); }}>
                    {a.path}
                  </button>
                </Td>
                <Td dim>{humanSize(a.size)}</Td>
                {kind === "audio" ? (
                  <Td><audio controls preload="none" style={{ height: 32 }} src={fileURL(a.path)} /></Td>
                ) : (
                  <>
                    <Td><Badge color={kindBadgeColor(a.kind)}>{a.kind}</Badge></Td>
                    <Td mono dim>{a.imageName || "-"}</Td>
                  </>
                )}
              </Tr>
            ))}
          </Tbl>
          {assets.length > limit && (
            <LoadMore onClick={() => setLimit(limit + 200)}>
              加载更多（还有 {assets.length - limit} 个）
            </LoadMore>
          )}
        </Card>
      )}
    </>
  );
}

interface TreeNode {
  name: string;
  path: string;
  dirs: Map<string, TreeNode>;
  files: { path: string; size: number; imageName?: string }[];
}

// 树视图行（tree-views 配方：chevron 旋转 + hover 行 + 缩进 guide 线）
function TreeRow({ node, depth, defaultOpen = false, onCtx }: {
  node: TreeNode; depth: number; defaultOpen?: boolean;
  onCtx: (path: string) => { label: string; icon?: React.ReactNode; onClick?: () => void }[];
}) {
  const openCtx = useOpenCtx();
  const [open, setOpen] = useState(defaultOpen || depth < 1);
  const dirCount = node.dirs.size + node.files.length;
  const hasChildren = dirCount > 0;
  return (
    <>
      {depth > 0 && (
        <button
          onClick={() => setOpen(!open)}
          onContextMenu={(e) => { e.preventDefault(); openCtx(e, onCtx(node.path)); }}
          className="flex w-full cursor-pointer select-none items-center gap-2 rounded-sm py-1.5 pr-2 text-left transition duration-100 ease-linear hover:bg-primary_hover"
          style={{ paddingLeft: (depth - 1) * 22 + 8 }}>
          <span className="flex size-4 items-center justify-center">
            {hasChildren ? <TreeChevron open={open} /> : null}
          </span>
          {open ? <IconFolderOpen size={16} className="text-fg-quaternary" /> : <IconFolder size={16} className="text-fg-quaternary" />}
          <span className="truncate text-sm font-semibold text-primary">{node.name}</span>
          <span className="ml-1 text-xs font-medium text-quaternary">{node.files.length}</span>
        </button>
      )}
      {open && (
        <>
          {[...node.dirs.values()].map((d) => (
            <TreeRow key={d.name} node={d} depth={depth + 1} onCtx={onCtx} />
          ))}
          {node.files.map((f) => (
            <FileRow key={f.path} file={f} depth={depth} onCtx={onCtx} />
          ))}
        </>
      )}
    </>
  );
}

function FileRow({ file, depth, onCtx }: {
  file: { path: string; size: number; imageName?: string };
  depth: number;
  onCtx: (path: string) => { label: string; icon?: React.ReactNode; onClick?: () => void }[];
}) {
  const openCtx = useOpenCtx();
  const kind = guessKind(file.path);
  return (
    <button
      onContextMenu={(e) => { e.preventDefault(); openCtx(e, onCtx(file.path)); }}
      onClick={() => window.open(fileURL(file.path), "_blank")}
      className="flex w-full cursor-pointer select-none items-center gap-2 rounded-sm py-1.5 pr-2 text-left transition duration-100 ease-linear hover:bg-primary_hover"
      style={{ paddingLeft: depth * 22 + 8 + 16 + 8 }}>
      <KindIcon kind={kind} />
      <span className="truncate text-sm text-secondary">{file.path.split("/").pop()}</span>
      {file.imageName && <span className="truncate font-mono text-xs text-quaternary">{file.imageName}</span>}
      <span className="ml-auto shrink-0 pl-2 text-xs font-medium text-quaternary">{humanSize(file.size)}</span>
    </button>
  );
}

// hooks 不能条件调用 —— 包装一下保证 TreeRow/FileRow 都在组件顶层使用
function useOpenCtx() {
  return useContextMenu();
}

function guessKind(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (["png", "jpg", "jpeg", "webp", "gif"].includes(ext)) return "image";
  if (["ogg", "opus", "mp3", "wav"].includes(ext)) return "audio";
  if (["ttf", "otf", "woff", "woff2"].includes(ext)) return "font";
  if (["rpy", "py", "txt"].includes(ext)) return "script";
  if (["webm", "mp4"].includes(ext)) return "video";
  return "other";
}

function KindIcon({ kind }: { kind: string }) {
  const cls = "size-4 shrink-0 text-fg-quaternary";
  switch (kind) {
    case "image": return <IconImage size={16} className={cls} />;
    case "audio": return <IconAudio size={16} className={cls} />;
    case "font": return <IconFont size={16} className={cls} />;
    case "script": return <IconScript size={16} className={cls} />;
    case "video": return <IconVideo size={16} className={cls} />;
    default: return <IconFile size={16} className={cls} />;
  }
}
