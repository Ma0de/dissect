// Untitled UI 风格线性图标（内联 SVG，20px viewBox，stroke 1.7 / 2.25 可调）
import React from "react";

function I({ d, children, size = 20, sw = 1.7 }: { d?: string; children?: React.ReactNode; size?: number; sw?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 20 20" fill="none"
      stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round">
      {d ? <path d={d} /> : children}
    </svg>
  );
}

export const IconHome = (p: any) => (
  <I {...p}><path d="M3.3 8.2 10 2.9l6.7 5.3v7.6a1.2 1.2 0 0 1-1.2 1.2h-3.2v-4.6H7.7v4.6H4.5a1.2 1.2 0 0 1-1.2-1.2V8.2Z" /></I>
);
export const IconChat = (p: any) => (
  <I {...p}><path d="M17 10.2a6.3 6.3 0 0 1-6.3 6.3c-.9 0-2.1-.2-2.9-.5L3.4 17l1-3.5c-.5-.9-.7-1.6-.7-2.6A6.3 6.3 0 0 1 10 4.6c3.9 0 7 1.7 7 5.6Z" /></I>
);
export const IconChoice = (p: any) => (
  <I {...p}><path d="M6 5h11M6 10h11M6 15h11" /><circle cx="3.2" cy="5" r="0.9" /><circle cx="3.2" cy="10" r="0.9" /><circle cx="3.2" cy="15" r="0.9" /></I>
);
export const IconFlow = (p: any) => (
  <I {...p}><circle cx="10" cy="4.5" r="2" /><circle cx="4.5" cy="15.5" r="2" /><circle cx="15.5" cy="15.5" r="2" /><path d="M10 6.5v3M10 9.5c0 2.5-5.5 2-5.5 4M10 9.5c0 2.5 5.5 2 5.5 4" /></I>
);
export const IconDB = (p: any) => (
  <I {...p}><ellipse cx="10" cy="4.8" rx="6" ry="2.3" /><path d="M4 4.8v10.4c0 1.3 2.7 2.3 6 2.3s6-1 6-2.3V4.8M4 10c0 1.3 2.7 2.3 6 2.3s6-1 6-2.3" /></I>
);
export const IconImage = (p: any) => (
  <I {...p}><rect x="3" y="3.5" width="14" height="13" rx="2" /><circle cx="7.3" cy="8" r="1.3" /><path d="m3.5 14 4-3.5 3 2.5 2.5-2 4 3" /></I>
);
export const IconCode = (p: any) => (
  <I {...p}><path d="m7 5-4.2 5L7 15M13 5l4.2 5L13 15" /></I>
);
export const IconSearch = (p: any) => (
  <I {...p}><circle cx="9" cy="9" r="5.5" /><path d="m13.2 13.2 3.8 3.8" /></I>
);
export const IconDownload = (p: any) => (
  <I {...p}><path d="M10 3.5v9m0 0 3.5-3.5M10 12.5 6.5 9M3.5 16.5h13" /></I>
);
export const IconCopy = (p: any) => (
  <I {...p}><rect x="7" y="7" width="10" height="10" rx="2" /><path d="M13 4.5H5A1.5 1.5 0 0 0 3.5 6v8" /></I>
);
export const IconCheck = (p: any) => <I {...p}><path d="m4 10.5 4 4L16 5.5" /></I>;
export const IconSpin = (p: any) => (
  <svg width={p?.size || 20} height={p?.size || 20} viewBox="0 0 20 20" fill="none" className="animate-spin-slow"
    stroke="currentColor" strokeWidth="1.7" strokeLinecap="round">
    <path d="M10 3a7 7 0 1 1-6.5 4.4" />
  </svg>
);
export const IconAlert = (p: any) => (
  <I {...p}><path d="M10 3.5 17.5 16h-15L10 3.5Z" /><path d="M10 8.5v3.2" /><circle cx="10" cy="13.9" r="0.4" fill="currentColor" /></I>
);
export const IconLayers = (p: any) => (
  <I {...p}><path d="m10 3 7 3.5L10 10 3 6.5 10 3Z" /><path d="m3 10 7 3.5 7-3.5M3 13.5 10 17l7-3.5" /></I>
);
export const IconUser = (p: any) => (
  <I {...p}><circle cx="10" cy="6.7" r="3.2" /><path d="M3.8 16.5c.8-3 3.2-4.5 6.2-4.5s5.4 1.5 6.2 4.5" /></I>
);
export const IconDoc = (p: any) => (
  <I {...p}><path d="M11.5 3H6a1.5 1.5 0 0 0-1.5 1.5v11A1.5 1.5 0 0 0 6 17h8a1.5 1.5 0 0 0 1.5-1.5V7L11.5 3Z" /><path d="M11.5 3v4h4" /></I>
);
export const IconPlay = (p: any) => <I {...p}><path d="M6 4.5v11l9-5.5-9-5.5Z" /></I>;
export const IconPause = (p: any) => <I {...p}><path d="M7 4.5v11M13 4.5v11" /></I>;
export const IconX = (p: any) => <I {...p} sw={2}><path d="m5 5 10 10M15 5 5 15" /></I>;

// ---- 新增（侧边栏 / 工具 / 文件） ----
export const IconChevronRight = (p: any) => <I {...p} sw={2.4}><path d="m7.5 4.5 5.5 5.5-5.5 5.5" /></I>;
export const IconChevronDown = (p: any) => <I {...p} sw={2.4}><path d="m4.5 7.5 5.5 5.5 5.5-5.5" /></I>;
export const IconChevronLeft = (p: any) => <I {...p} sw={2.4}><path d="m12.5 4.5-5.5 5.5 5.5 5.5" /></I>;
export const IconFolder = (p: any) => (
  <I {...p}><path d="M3 5.5A1.5 1.5 0 0 1 4.5 4h3.2l1.8 2h6A1.5 1.5 0 0 1 17 7.5v7a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 3 14.5v-9Z" /></I>
);
export const IconFolderOpen = (p: any) => (
  <I {...p}><path d="M3 6a1.5 1.5 0 0 1 1.5-1.5h3.2l1.8 2h6A1.5 1.5 0 0 1 17 8v.8M3 6v8.5A1.5 1.5 0 0 0 4.5 16h10a1.5 1.5 0 0 0 1.44-1.08L17.6 9.7a.9.9 0 0 0-.86-1.2H4.6" /></I>
);
export const IconReveal = (p: any) => (
  <I {...p}><path d="M3 5.5A1.5 1.5 0 0 1 4.5 4h3.2l1.8 2h6A1.5 1.5 0 0 1 17 7.5V10M3 5.5v9A1.5 1.5 0 0 0 4.5 16h4.3" /><path d="M14.5 12.5v5m0 0-2.2-2.2m2.2 2.2 2.2-2.2" /></I>
);
export const IconTrash = (p: any) => (
  <I {...p}><path d="M4 5.5h12M8 5.5V4.2A1.2 1.2 0 0 1 9.2 3h1.6A1.2 1.2 0 0 1 12 4.2v1.3M5.5 5.5l.7 9.6A1.5 1.5 0 0 0 7.7 16.5h4.6a1.5 1.5 0 0 0 1.5-1.4l.7-9.6" /></I>
);
export const IconMoon = (p: any) => <I {...p}><path d="M17 10.75A7.5 7.5 0 0 1 9.25 3a5.75 5.75 0 1 0 7.75 7.75Z" /></I>;
export const IconSun = (p: any) => (
  <I {...p}><circle cx="10" cy="10" r="3.5" /><path d="M10 1.5v2M10 16.5v2M1.5 10h2M16.5 10h2M4 4l1.4 1.4M14.6 14.6 16 16M16 4l-1.4 1.4M5.4 14.6 4 16" /></I>
);
export const IconUploadCloud = (p: any) => (
  <I {...p}><path d="M6.3 15.5H5a3.2 3.2 0 0 1-.5-6.36 4.6 4.6 0 0 1 9-.84A3.6 3.6 0 0 1 15 15.5h-1.3" /><path d="M10 16.5V9.5m0 0L7.5 12M10 9.5 12.5 12" /></I>
);
export const IconRefresh = (p: any) => (
  <I {...p}><path d="M16.5 10a6.5 6.5 0 1 1-1.9-4.6M16.5 3.5v3h-3" /></I>
);
export const IconMoreVertical = (p: any) => (
  <I {...p} sw={2}><circle cx="10" cy="4.5" r="0.5" fill="currentColor" /><circle cx="10" cy="10" r="0.5" fill="currentColor" /><circle cx="10" cy="15.5" r="0.5" fill="currentColor" /></I>
);
export const IconExternal = (p: any) => (
  <I {...p}><path d="M8.5 4.5H5A1.5 1.5 0 0 0 3.5 6v9A1.5 1.5 0 0 0 5 16.5h9a1.5 1.5 0 0 0 1.5-1.5v-3.5" /><path d="M12 3.5h4.5V8M16.2 3.8 9.5 10.5" /></I>
);
export const IconInfo = (p: any) => (
  <I {...p}><circle cx="10" cy="10" r="7" /><path d="M10 9v4.5" /><circle cx="10" cy="6.4" r="0.5" fill="currentColor" /></I>
);
export const IconCheckCircle = (p: any) => (
  <I {...p}><circle cx="10" cy="10" r="7" /><path d="m6.8 10.2 2.2 2.2 4.2-4.6" /></I>
);
export const IconFile = (p: any) => (
  <I {...p}><path d="M11.5 3H6a1.5 1.5 0 0 0-1.5 1.5v11A1.5 1.5 0 0 0 6 17h8a1.5 1.5 0 0 0 1.5-1.5V7L11.5 3Z" /><path d="M11.5 3v4h4" /></I>
);
export const IconFileZip = (p: any) => (
  <I {...p}><path d="M11.5 3H6a1.5 1.5 0 0 0-1.5 1.5v11A1.5 1.5 0 0 0 6 17h8a1.5 1.5 0 0 0 1.5-1.5V7L11.5 3Z" /><path d="M11.5 3v4h4M8.5 6h1M8.5 8.5h1M8.5 11h1M8.5 13.5h3v3h-3z" /></I>
);
export const IconAudio = (p: any) => (
  <I {...p}><path d="M8 15.5V5l7-1.5V14" /><circle cx="6" cy="15.5" r="2" /><circle cx="13" cy="14" r="2" /></I>
);
export const IconVideo = (p: any) => (
  <I {...p}><rect x="2.5" y="4.5" width="15" height="11" rx="2" /><path d="m8 8 4 2-4 2z" /></I>
);
export const IconFont = (p: any) => (
  <I {...p}><path d="M4 16 9 4h2l5 12M6.3 11.5h7.4" /></I>
);
export const IconScript = (p: any) => <IconCode {...p} />;
export const IconGrid = (p: any) => (
  <I {...p}><rect x="3.5" y="3.5" width="5.5" height="5.5" rx="1" /><rect x="11" y="3.5" width="5.5" height="5.5" rx="1" /><rect x="3.5" y="11" width="5.5" height="5.5" rx="1" /><rect x="11" y="11" width="5.5" height="5.5" rx="1" /></I>
);
export const IconList = (p: any) => (
  <I {...p}><path d="M7 5h10M7 10h10M7 15h10" /><circle cx="3.8" cy="5" r="0.6" fill="currentColor" /><circle cx="3.8" cy="10" r="0.6" fill="currentColor" /><circle cx="3.8" cy="15" r="0.6" fill="currentColor" /></I>
);
export const IconSettings = (p: any) => (
  <I {...p}><circle cx="10" cy="10" r="2.6" /><path d="M10 2.8v2M10 15.2v2M2.8 10h2M15.2 10h2M4.9 4.9l1.4 1.4M13.7 13.7l1.4 1.4M15.1 4.9l-1.4 1.4M6.3 13.7l-1.4 1.4" /></I>
);
