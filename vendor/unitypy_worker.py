#!/usr/bin/env python3
"""UnityPy 资产提取 worker（可选模块）。

用法: python3 unitypy_worker.py <游戏根目录> <输出assets目录>
依赖: pip install UnityPy

遍历 *_Data 目录中的 Unity 资产文件 (level/sharedassets/*.assets、.assets.resS、
*.bundle)，导出 Texture2D/Sprite (PNG)、AudioClip (OGG/WAV)、TextAsset、Font。
"""
import sys, os, traceback

def main():
    root, out = sys.argv[1], sys.argv[2]
    try:
        import UnityPy
    except ImportError:
        print("UnityPy 未安装: pip install UnityPy")
        sys.exit(2)

    n = 0
    targets = []
    for dirpath, dirnames, filenames in os.walk(root):
        for fn in filenames:
            if fn.endswith((".assets", ".assets.resS", ".resource", ".bundle", ".assets bundles")):
                targets.append(os.path.join(dirpath, fn))
    for t in targets:
        try:
            env = UnityPy.load(t)
            for obj in env.objects:
                try:
                    if obj.type.name in ("Texture2D", "Sprite"):
                        data = obj.read()
                        img = data.image
                        if img is not None:
                            name = getattr(data, "m_Name", None) or f"{obj.type.name}_{obj.path_id}"
                            dst = os.path.join(out, "images", f"{name}.png")
                            os.makedirs(os.path.dirname(dst), exist_ok=True)
                            img.save(dst)
                            n += 1
                    elif obj.type.name == "AudioClip":
                        data = obj.read()
                        for name, sample in data.samples.items():
                            dst = os.path.join(out, "audio", os.path.basename(name))
                            os.makedirs(os.path.dirname(dst), exist_ok=True)
                            with open(dst, "wb") as f:
                                f.write(sample)
                            n += 1
                    elif obj.type.name == "TextAsset":
                        data = obj.read()
                        dst = os.path.join(out, "textassets", f"{data.m_Name}.txt")
                        os.makedirs(os.path.dirname(dst), exist_ok=True)
                        script = data.m_Script
                        if isinstance(script, str):
                            script = script.encode("utf-8", "surrogateescape")
                        with open(dst, "wb") as f:
                            f.write(script)
                        n += 1
                except Exception:
                    continue
        except Exception:
            traceback.print_exc()
    print(f"extracted {n} objects from {len(targets)} files")

if __name__ == "__main__":
    main()
