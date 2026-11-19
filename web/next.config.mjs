/** @type {import('next').NextConfig} */
const nextConfig = {
  // 静态导出：构建产物 out/ 由 Go 服务器 (go:embed) 直接托管
  output: "export",
  images: { unoptimized: true },
  trailingSlash: true,
};

export default nextConfig;
