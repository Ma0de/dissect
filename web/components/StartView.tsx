"use client";
import { useState } from "react";
import { useModel } from "./Shell";
import { Button, Input } from "./ui";

export default function StartView() {
  const { openPath, busy, modelErr, state, addToast } = useModel();
  const [path, setPath] = useState("");
  const [going, setGoing] = useState(false);
  const showErr = state?.mode === "browse";

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const p = path.trim().replace(/^"|"$/g, "");
    if (going || busy) return;
    if (!p) {
      addToast("error", "请输入游戏路径，或把游戏文件夹拖进来");
      return;
    }
    setGoing(true);
    try {
      await openPath(p);
    } finally {
      setGoing(false);
    }
  };

  return (
    <div className="flex flex-col items-center pt-[14vh]">
      <div className="w-full max-w-[720px]">
        <h1 className="text-center text-[32px] font-semibold leading-tight tracking-[-0.4px] text-primary">
          打开游戏文件夹
        </h1>
        <p className="mt-4 text-center text-base text-tertiary">
          支持 Ren&apos;Py（game/ 目录、zip、.rpa）与 Unity IL2CPP；远程访问时拖拽即上传
        </p>
        <form className="mt-10" onSubmit={submit}>
          <div className="flex w-full items-center gap-2 rounded-lg bg-primary shadow-xs ring-1 ring-primary ring-inset transition duration-100 ease-linear focus-within:ring-2 focus-within:ring-brand">
            <Input
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="输入本地路径..."
              spellCheck={false}
              autoComplete="off"
              className="h-10 px-3.5 text-sm"
            />
            <Button type="submit" variant="primary" size="sm" className="mr-1 shrink-0"
              disabled={going || busy}>
              {going ? "打开中…" : "解包"}
            </Button>
          </div>
        </form>
        {showErr && modelErr && (
          <p className="mt-4 text-center text-xs font-medium text-fg-error-primary">{modelErr}</p>
        )}
      </div>
    </div>
  );
}
