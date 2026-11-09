module.exports = {
  testDir: ".",
  testMatch: "*.spec.cjs",
  workers: 1,
  reporter: "line",
  use: { baseURL: "http://127.0.0.1:5175", headless: true },
  webServer: {
    command: "bun ../scripts/preview.ts",
    url: "http://127.0.0.1:5175",
    reuseExistingServer: false,
  },
};
