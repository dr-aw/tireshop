module.exports = {
  testDir: ".",
  testMatch: "*.spec.cjs",
  workers: 1,
  timeout: 30000,
  reporter: "line",
  outputDir: "../../.tmp/browser-results",
  use: {
    baseURL: process.env.BASE_URL,
    browserName: "chromium",
    headless: true,
  },
};
