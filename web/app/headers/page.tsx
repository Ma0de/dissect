// 头文件页：Model.h / il2cpp.h / dump.cs 查看
"use client";
import { useEffect, useState } from "react";
import { useModel } from "../../components/Shell";
import {
  Card, Loading, EmptyState, PageHead, Button, CopyBtn, CodeSnippet,
} from "../../components/ui";
import { exportURL } from "../../lib/api";

export default function Headers() {
  const { model, modelErr: err } = useModel();
  const [current, setCurrent] = useState<string>("");
  const [content, setContent] = useState<string>("");
  const [loading, setLoading] = useState(false);

  const headers = model?.headers || [];

  useEffect(() => {
    if (headers.length && !current) setCurrent(headers[0].name);
  }, [headers, current]);

  useEffect(() => {
    if (!current) return;
    setLoading(true);
    fetch("/api/headers/" + encodeURIComponent(current))
      .then((r) => r.text())
      .then(setContent)
      .catch(() => setContent("// 读取失败"))
      .finally(() => setLoading(false));
  }, [current]);

  if (err) return <EmptyState title={`读取失败：${err}`} />;
  if (!model) return <Loading />;
  if (!headers.length) {
    return (
      <>
        <PageHead title="头文件" />
        <EmptyState
          icon={<IconCode />}
          title="尚未生成头文件"
          sub="运行 dissect unpack 后会自动生成 Model.h；Unity IL2CPP 游戏搭配 Il2CppDumper 会生成 il2cpp.h / dump.cs" />
      </>
    );
  }
  const meta = headers.find((h) => h.name === current);

  return (
    <>
      <PageHead
        title="头文件"
        sub="类 IL2CPP.h 的 C++ 数据模型，供第三方开发者做字段扫描与修改软件"
        actions={
          <>
            <CopyBtn text={content} label="复制内容" />
            <Button variant="primary" size="sm" onClick={() => location.assign(exportURL(current))}>
              下载 {current}
            </Button>
          </>
        }
      />
      <div className="mb-4 flex flex-wrap gap-1.5">
        {headers.map((h) => (
          <button key={h.name} onClick={() => setCurrent(h.name)}
            className={`inline-flex cursor-pointer items-center rounded-md px-2.5 py-1.5 text-sm font-semibold transition duration-100 ease-linear ${
              h.name === current
                ? "bg-primary_hover text-primary"
                : "text-tertiary hover:bg-primary_hover hover:text-secondary"
            }`}>
            <span className="font-mono">{h.name}</span>
          </button>
        ))}
      </div>
      {meta?.note && <p className="mb-4 text-sm text-tertiary">{meta.note}</p>}
      <CodeSnippet code={loading ? "加载中…" : content} maxH="max-h-[640px]" />
    </>
  );
}

function IconCode() {
  return (
    <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round"
      strokeLinejoin="round" width="20" height="20">
      <path d="m7 5-4.2 5L7 15M13 5l4.2 5L13 15" />
    </svg>
  );
}
