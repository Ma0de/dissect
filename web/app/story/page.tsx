"use client";
import { useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useModel } from "../../components/Shell";
import {
  Card, Loading, EmptyState, SearchInput, Avatar, PageHead, Button,
  Badge, LoadMore, Toggle, useContextMenu,
} from "../../components/ui";
import { exportURL, fileURL } from "../../lib/api";
import { IconChat, IconCopy, IconPlay, IconPause, IconScript } from "../../components/icons";

export default function Story() {
  const { model, modelErr: err, addToast } = useModel();
  const router = useRouter();
  const [char, setChar] = useState<string>("");
  const [q, setQ] = useState("");
  const [limit, setLimit] = useState(300);
  const [voiceOnly, setVoiceOnly] = useState(false);
  const [playing, setPlaying] = useState<string | null>(null);
  const audioRef = useRef<HTMLAudioElement>(null);
  const openCtx = useContextMenu();

  const chars = useMemo(() => {
    if (!model) return [];
    return model.characters.filter((c) => c.lineCount > 0);
  }, [model]);

  const voiceMapped = useMemo(() => {
    if (!model) return 0;
    return model.dialogues.filter((d) => !!d.audio).length;
  }, [model]);

  const dialogues = useMemo(() => {
    if (!model) return [];
    let ds = model.dialogues;
    if (char) ds = ds.filter((d) => d.speaker === char);
    if (q.trim()) {
      const kw = q.trim().toLowerCase();
      ds = ds.filter((d) => d.text.toLowerCase().includes(kw) || (d.label || "").includes(kw));
    }
    if (voiceOnly) ds = ds.filter((d) => !!d.audio);
    return ds;
  }, [model, char, q, voiceOnly]);

  if (err) return <EmptyState title={`读取失败：${err}`} />;
  if (!model) return <Loading />;
  const active = chars.find((c) => c.tag === char);
  const charName = (tag: string) => model.characters.find((c) => c.tag === tag)?.name || tag;

  const play = (src: string) => {
    const el = audioRef.current;
    if (!el) return;
    if (playing === src) {
      el.pause();
      setPlaying(null);
      return;
    }
    el.src = fileURL(src);
    el.play().then(() => setPlaying(src)).catch(() => addToast("error", "无法播放该语音"));
  };

  return (
    <>
      <audio ref={audioRef} className="hidden" preload="none" onEnded={() => setPlaying(null)} />
      <PageHead
        title="剧情文案"
        sub={`共 ${model.dialogues.length} 句台词 · ${chars.length} 个角色 · ${voiceMapped} 句已映射语音 · 含 {标签} 与 [变量] 原文`}
        actions={
          <>
            <div className="mr-2 flex items-center gap-2.5">
              <span className="text-sm font-medium text-tertiary">仅看有语音</span>
              <Toggle checked={voiceOnly} onChange={(v) => { setVoiceOnly(v); setLimit(300); }} />
            </div>
            <Button variant="secondary" size="sm" onClick={() => location.assign(exportURL("dialogues.csv"))}>导出 CSV</Button>
            <Button variant="secondary" size="sm" onClick={() => location.assign(exportURL("dialogues_by_character.txt"))}>按角色分组 TXT</Button>
            <Button variant="secondary" size="sm" onClick={() => location.assign(exportURL("dialogues.json"))}>导出 JSON</Button>
          </>
        }
      />
      <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-[260px_minmax(0,1fr)]">
        <Card title="角色" flush className="lg:sticky lg:top-0">
          <div className="p-2">
            <button
              onClick={() => { setChar(""); setLimit(300); }}
              className={`flex w-full cursor-pointer select-none items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-semibold transition duration-100 ease-linear ${
                char === "" ? "bg-primary_hover text-primary" : "text-tertiary hover:bg-primary_hover hover:text-secondary"
              }`}>
              <span className="flex-1 truncate text-left">全部台词</span>
              <span className="text-xs font-medium text-quaternary">{model.dialogues.length}</span>
            </button>
            {chars.map((c) => (
              <button key={c.tag}
                onClick={() => { setChar(c.tag); setLimit(300); }}
                className={`flex w-full cursor-pointer select-none items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-semibold transition duration-100 ease-linear ${
                  char === c.tag ? "bg-primary_hover text-primary" : "text-tertiary hover:bg-primary_hover hover:text-secondary"
                }`}>
                <Avatar name={c.name || c.tag} color={c.color} />
                <span className="flex-1 truncate text-left">{c.name || c.tag}</span>
                <span className="text-xs font-medium text-quaternary">{c.lineCount}</span>
              </button>
            ))}
          </div>
        </Card>
        <Card
          title={active ? `【${active.name}】的台词` : "全部台词"}
          extra={<div className="w-full max-w-[300px]"><SearchInput value={q} onChange={setQ} placeholder="搜索台词内容 / 章节名…" /></div>}
          flush
        >
          {dialogues.length === 0 ? (
            <EmptyState icon={<IconChat />} title="没有匹配的台词" />
          ) : (
            <div>
              {dialogues.slice(0, limit).map((d) => (
                <div className="flex gap-3 border-b border-secondary px-5 py-3.5 transition-colors duration-100 ease-linear hover:bg-primary_hover"
                  key={d.id}
                  onContextMenu={(e) => openCtx(e, [
                    { label: "复制台词文本", icon: <IconCopy />, onClick: () => { navigator.clipboard?.writeText(d.text).catch(() => {}); addToast("success", "台词文本已复制"); } },
                    {
                      label: `定位 ${d.file}:${d.line}`, icon: <IconScript />,
                      onClick: () => router.push(`/script?file=${encodeURIComponent(d.file)}&line=${d.line}`),
                    },
                  ])}>
                  <div className="w-36 shrink-0">
                    <div className="flex items-center gap-2">
                      <Avatar name={d.speaker === "narrator" ? "旁" : d.speaker} />
                      <div className="truncate text-sm font-medium text-primary">
                        {charName(d.speaker)}
                      </div>
                    </div>
                    <button
                      className="mt-1 block max-w-full cursor-pointer truncate text-left font-mono text-xs text-quaternary underline decoration-transparent underline-offset-2 transition duration-100 ease-linear hover:text-fg-brand-secondary hover:decoration-current"
                      title={`在脚本中定位 ${d.file}:${d.line}`}
                      onClick={() => router.push(`/script?file=${encodeURIComponent(d.file)}&line=${d.line}`)}>
                      {d.file.split("_").slice(1).join("_") || d.file}:{d.line}
                    </button>
                  </div>
                  <div className="min-w-0 flex-1 whitespace-pre-wrap text-sm leading-relaxed text-primary">
                    {d.text}
                    {d.cond && <Badge color="warning" className="ml-2 align-middle">条件: {d.cond}</Badge>}
                    {d.label && <Badge color="gray" className="ml-2 align-middle">#{d.label}</Badge>}
                    {d.audio && (
                      <div className="mt-1.5 flex flex-wrap items-center gap-2">
                        <button onClick={() => play(d.audio!)}
                          className={`inline-flex cursor-pointer items-center gap-1 rounded-md px-1.5 py-0.5 text-xs font-semibold transition duration-100 ease-linear ${
                            playing === d.audio
                              ? "bg-brand-secondary/20 text-fg-brand-primary"
                              : "text-tertiary hover:bg-primary_hover hover:text-secondary"
                          }`}>
                          {playing === d.audio ? <IconPause size={13} /> : <IconPlay size={13} />}
                          {playing === d.audio ? "暂停" : "试听"}
                        </button>
                        <span className="truncate font-mono text-[11px] text-quaternary" title={d.audio}>
                          {d.audio.split("/").pop()}
                        </span>
                      </div>
                    )}
                  </div>
                </div>
              ))}
              {dialogues.length > limit && (
                <LoadMore onClick={() => setLimit(limit + 500)}>
                  加载更多（还有 {dialogues.length - limit} 句）
                </LoadMore>
              )}
            </div>
          )}
        </Card>
      </div>
    </>
  );
}
