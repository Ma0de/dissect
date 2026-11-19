// 流程图页：React Flow 分层布局 + Mermaid 导出
"use client";
import { useMemo, useState, useEffect } from "react";
import {
  ReactFlow, Background, Controls, MiniMap, type Node, type Edge,
  MarkerType,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useModel } from "../../components/Shell";
import {
  Card, Loading, EmptyState, PageHead, Badge, Button, CopyBtn, Toggle,
} from "../../components/ui";
import { exportURL, type FlowEdge } from "../../lib/api";

function toMermaid(nodes: { id: string; label: string; kind: string }[],
  edges: FlowEdge[]): string {
  const safe = (s: string) => "n_" + s.replace(/[^A-Za-z0-9]/g, "_");
  const esc = (s: string) => s.replace(/"/g, "'").replace(/\|/g, "/").replace(/\n/g, " ").slice(0, 57);
  let out = "flowchart TD\n";
  for (const n of nodes) {
    out += n.kind === "screen"
      ? `  ${safe(n.id)}[[${esc(n.label)} (screen)]]\n`
      : `  ${safe(n.id)}[${esc(n.label)}]\n`;
  }
  for (const e of edges) {
    const lbl = e.label || (e.cond ? `if ${e.cond}` : "");
    const mark = e.kind.startsWith("call") ? "-.->" : "-->";
    out += lbl
      ? `  ${safe(e.from)} ${mark}|${esc(lbl)}| ${safe(e.to)}\n`
      : `  ${safe(e.from)} ${mark} ${safe(e.to)}\n`;
  }
  return out;
}

export default function Flow() {
  const { model, modelErr: err } = useModel();
  const [focus, setFocus] = useState<string>("");
  const [showScreens, setShowScreens] = useState(false);

  useEffect(() => {
    const sp = new URLSearchParams(window.location.search);
    if (sp.get("focus")) setFocus(sp.get("focus")!);
  }, []);

  const { nodes, edges } = useMemo(() => {
    if (!model) return { nodes: [] as Node[], edges: [] as Edge[] };
    const allN = model.flow.nodes;
    const allE = model.flow.edges;
    const fn = showScreens ? allN : allN.filter((n) => n.kind !== "screen");
    const fnIds = new Set(fn.map((n) => n.id));
    const fe = showScreens ? allE : allE.filter((e) => fnIds.has(e.from) && fnIds.has(e.to));

    // 分层布局：主线 label 依 BFS 分层横向排布；screen 独立成列在左侧
    const labelNodes = fn.filter((n) => n.kind !== "screen");
    const screenNodes = fn.filter((n) => n.kind === "screen");
    const isLabel = new Set(labelNodes.map((n) => n.id));

    const levelOf: Record<string, number> = {};
    const roots = labelNodes.filter((n) => !fe.some((e) => e.to === n.id && isLabel.has(e.from))).map((n) => n.id);
    if (!roots.length && labelNodes.length) roots.push(labelNodes[0].id);
    const queue = [...roots];
    roots.forEach((r) => (levelOf[r] = 0));
    while (queue.length) {
      const cur = queue.shift()!;
      const lv = (levelOf[cur] ?? 0) + 1;
      for (const e of fe) {
        if (e.from === cur && isLabel.has(e.to) && levelOf[e.to] === undefined) {
          levelOf[e.to] = lv;
          queue.push(e.to);
        }
      }
    }
    const byLevel: Record<number, string[]> = {};
    for (const n of labelNodes) {
      const lv = levelOf[n.id] ?? 0;
      (byLevel[lv] ||= []).push(n.id);
    }
    const nodePos: Record<string, { x: number; y: number }> = {};
    const levelCount = Math.max(...Object.keys(byLevel).map(Number), 0);
    const COL_W = 270, ROW_H = 130;
    for (const lvStr of Object.keys(byLevel)) {
      const lv = Number(lvStr);
      const ids = byLevel[lv];
      ids.forEach((id, i) => {
        nodePos[id] = { x: (i - ids.length / 2) * COL_W + 420, y: 40 + lv * ROW_H };
      });
    }
    screenNodes.forEach((n, i) => {
      const col = Math.floor(i / 5), row = i % 5;
      nodePos[n.id] = { x: 100 + col * 250, y: 40 + (levelCount + 1.6) * ROW_H + row * 104 };
    });

    const nodes: Node[] = fn.map((n) => ({
      id: n.id,
      position: nodePos[n.id] || { x: 0, y: 0 },
      data: { label: (n.kind === "screen" ? "settings " : "") + n.label },
      className: n.kind === "screen"
        ? "!rounded-lg !bg-utility-brand-50 !ring-1 !ring-utility-brand-200 !px-3.5 !py-2 !text-sm !font-medium !text-utility-brand-700"
        : "!rounded-lg !bg-primary !ring-1 !ring-primary !px-3.5 !py-2 !text-sm !font-medium !text-primary",
      clickable: true,
    }));
    const edges: Edge[] = fe.map((e, i) => ({
      id: `e${i}`,
      source: e.from,
      target: e.to,
      label: e.label || (e.cond ? `if ${e.cond}` : undefined),
      animated: e.kind === "choice",
      style: {
        stroke: e.kind === "choice" ? "#7f56d9"
          : e.kind.startsWith("call") ? "#cd8900" : "#a1a1a1",
        strokeWidth: 1.6,
      },
      markerEnd: { type: MarkerType.ArrowClosed, color: e.kind === "choice" ? "#7f56d9" : "#a1a1a1" },
    }));
    return { nodes, edges };
  }, [model, showScreens]);

  if (err) return <EmptyState title={`读取失败：${err}`} />;
  if (!model) return <Loading />;

  const mmd = toMermaid(model.flow.nodes, model.flow.edges);
  const focused = focus ? model.flow.nodes.find((n) => n.id === focus) : null;

  return (
    <>
      <PageHead
        title="整体流程图"
        sub={`${model.flow.nodes.length} 个节点（label + screen） · ${model.flow.edges.length} 条边（jump/call/choice）`}
        actions={
          <>
            <div className="mr-2 flex items-center gap-2.5">
              <span className="text-sm font-medium text-tertiary">screen 节点</span>
              <Toggle checked={showScreens} onChange={setShowScreens} />
            </div>
            <Button variant="secondary" size="sm" onClick={() => location.assign(exportURL("flow.mmd"))}>下载 Mermaid</Button>
            <CopyBtn text={mmd} label="复制 Mermaid 源码" />
            <Button variant="secondary" size="sm" onClick={() => location.assign(exportURL("flow.json"))}>flow.json</Button>
          </>
        }
      />
      {focused && (
        <div className="mb-3 flex items-center gap-2">
          <Badge color="brand">选中: {focused.label}</Badge>
          {focused.file && <span className="font-mono text-xs text-quaternary">{focused.file}:{focused.line}</span>}
        </div>
      )}
      <Card flush>
        <div className="h-[calc(100vh-240px)] min-h-[520px] overflow-hidden rounded-xl bg-secondary_alt">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            fitView
            minZoom={0.15}
            nodesDraggable
            onNodeClick={(_, n) => setFocus(n.id)}
            proOptions={{ hideAttribution: true }}
          >
            <Background gap={22} />
            <Controls />
            <MiniMap pannable zoomable nodeColor={() => "#d6bbfb"} />
          </ReactFlow>
        </div>
      </Card>
    </>
  );
}
