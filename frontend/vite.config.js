// The CLI is versioned by mise. API targets can be replaced by the test harness.
export default {
  define: {
    __PUBLIC_MEDIA_BASE__: JSON.stringify(
      process.env.VITE_MEDIA_BASE_URL || "",
    ),
  },
  build: {
    rollupOptions: {
      input: {
        main: "frontend/index.html",
        information: "frontend/information.html",
        administrator: "frontend/administrator.html",
      },
    },
  },
  server: {
    allowedHosts: ["dev"],
    proxy: {
      "/api/admin/products":
        process.env.CATALOG_PROXY_TARGET || "http://127.0.0.1:8081",
      "/api/admin":
        process.env.REQUESTS_PROXY_TARGET || "http://127.0.0.1:8082",
      "/api/products":
        process.env.CATALOG_PROXY_TARGET || "http://127.0.0.1:8081",
      "/api/requests":
        process.env.REQUESTS_PROXY_TARGET || "http://127.0.0.1:8082",
    },
  },
};
