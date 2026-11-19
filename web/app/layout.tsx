import type { Metadata } from "next";
import Shell from "../components/Shell";
import "./globals.css";

export const metadata: Metadata = {
  title: "dissect",
  description: "通用游戏解包与逆向分析工具",
};

// 主题在首帧前就写入 .dark-mode（与 untitledui.com 相同的 class 挂法），避免闪白
const themeInit = `(function(){try{var t=localStorage.getItem("dissect-theme");var d=t?t==="dark":(window.matchMedia&&window.matchMedia("(prefers-color-scheme: dark)").matches);if(d)document.documentElement.classList.add("dark-mode");}catch(e){}})();`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInit }} />
      </head>
      <body>
        <Shell>{children}</Shell>
      </body>
    </html>
  );
}
