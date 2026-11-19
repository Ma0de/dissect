// 字段页：store / persistent / 类字段 / screen 参数
"use client";
import { useMemo, useState } from "react";
import { useModel } from "../../components/Shell";
import {
  Card, Loading, EmptyState, SearchInput, Badge, PageHead, Button,
  kindBadgeColor, LoadMore, Tbl, Tr, Td,
} from "../../components/ui";
import { exportURL } from "../../lib/api";
import { IconDB } from "../../components/icons";

function scopeGroup(scope: string): string {
  if (scope === "store") return "store";
  if (scope === "persistent") return "persistent";
  if (scope.startsWith("class:") || scope.startsWith("ns:")) return "class";
  if (scope.startsWith("screen:")) return "screen";
  if (scope.startsWith("character:")) return "character";
  return "other";
}

export default function Fields() {
  const { model, modelErr: err } = useModel();
  const [group, setGroup] = useState("all");
  const [q, setQ] = useState("");
  const [limit, setLimit] = useState(200);

  const counts = useMemo(() => {
    if (!model) return {} as Record<string, number>;
    const c: Record<string, number> = { all: model.fields.length };
    for (const f of model.fields) {
      const g = scopeGroup(f.scope);
      c[g] = (c[g] || 0) + 1;
    }
    return c;
  }, [model]);

  const fields = useMemo(() => {
    if (!model) return [];
    let fs = model.fields;
    if (group !== "all") fs = fs.filter((f) => scopeGroup(f.scope) === group);
    if (q.trim()) {
      const kw = q.trim().toLowerCase();
      fs = fs.filter((f) =>
        f.name.toLowerCase().includes(kw) ||
        f.scope.toLowerCase().includes(kw) ||
        (f.default || "").toLowerCase().includes(kw));
    }
    return fs;
  }, [model, group, q]);

  if (err) return <EmptyState title={`读取失败：${err}`} />;
  if (!model) return <Loading />;

  return (
    <>
      <PageHead
        title="字段"
        sub={`${model.fields.length} 个字段`}
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={() => location.assign(exportURL("fields.csv"))}>导出 CSV</Button>
            <Button variant="secondary" size="sm" onClick={() => location.assign(exportURL("fields.json"))}>导出 JSON</Button>
            <Button variant="primary" size="sm" onClick={() => location.assign("/headers")}>Model.h</Button>
          </>
        }
      />
      <div className="mb-4 max-w-[480px]">
        <SearchInput value={q} onChange={setQ} placeholder="搜索字段名 / 作用域 / 默认值…" />
      </div>
      <div className="mb-4">
        <Card flush>
          <div className="flex flex-wrap gap-1 p-2">
            {[
              { key: "all", label: "全部", count: counts.all },
              { key: "store", label: "store 变量", count: counts.store || 0 },
              { key: "persistent", label: "persistent", count: counts.persistent || 0 },
              { key: "class", label: "类字段", count: counts.class || 0 },
              { key: "screen", label: "screen 参数", count: counts.screen || 0 },
              { key: "character", label: "角色定义", count: counts.character || 0 },
              { key: "other", label: "其他", count: counts.other || 0 },
            ].map((t) => (
              <button key={t.key} onClick={() => { setGroup(t.key); setLimit(200); }}
                className={`inline-flex cursor-pointer items-center gap-1.5 rounded-md px-2.5 py-1.5 text-sm font-semibold transition duration-100 ease-linear ${
                  group === t.key
                    ? "bg-primary_hover text-primary"
                    : "text-tertiary hover:bg-primary_hover hover:text-secondary"
                }`}>
                {t.label}
                <span className="text-xs font-medium text-quaternary">{t.count}</span>
              </button>
            ))}
          </div>
        </Card>
      </div>
      <Card flush>
        {fields.length === 0 ? (
          <EmptyState icon={<IconDB />} title="没有匹配的字段" />
        ) : (
          <>
            <Tbl cols={["字段名", "作用域", "类型", "默认值 / 偏移", "来源", "定义方式"]}>
              {fields.slice(0, limit).map((f, i) => (
                <Tr key={i}>
                  <Td mono className="text-primary">{f.name}</Td>
                  <Td><Badge color={kindBadgeColor(scopeGroup(f.scope))}>{f.scope}</Badge></Td>
                  <Td><Badge color="gray">{f.type}</Badge></Td>
                  <Td mono dim className="max-w-[220px] truncate">{f.default || "-"}</Td>
                  <Td mono dim>{f.file}:{f.line}</Td>
                  <Td dim>{f.keyword}</Td>
                </Tr>
              ))}
            </Tbl>
            {fields.length > limit && (
              <LoadMore onClick={() => setLimit(limit + 300)}>
                加载更多（还有 {fields.length - limit} 行）
              </LoadMore>
            )}
          </>
        )}
      </Card>
    </>
  );
}
