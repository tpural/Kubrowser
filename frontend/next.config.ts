import type { NextConfig } from "next";
import * as dotenv from "dotenv";
import path from "path";

// Load .env from root directory
dotenv.config({ path: path.resolve(__dirname, "../.env") });

const nextConfig: NextConfig = {
  images: {
    remotePatterns: [],
    unoptimized: false,
  },
};

export default nextConfig;
