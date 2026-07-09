/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // @fort/gateway-shared ships raw TypeScript (its `main` is src/index.ts), so
  // Next must transpile it rather than expect a prebuilt package.
  transpilePackages: ["@fort/gateway-shared"],
};

export default nextConfig;
