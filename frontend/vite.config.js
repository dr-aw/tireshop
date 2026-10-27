// The CLI is versioned by mise. API targets can be replaced by the test harness.
export default {
  server: {
    allowedHosts: ["dev"],
    proxy: {
      "/api/products":
        process.env.CATALOG_PROXY_TARGET || "http://127.0.0.1:8081",
      "/api/requests":
        process.env.REQUESTS_PROXY_TARGET || "http://127.0.0.1:8082",
    },
  },
};
