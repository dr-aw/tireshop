import { pathToFileURL } from "node:url";

// Dependency injection is for offline tests; CLI always uses the official endpoint.
export async function publishContent(
  { service, commit, token },
  {
    request = fetch,
    sleep = (ms) => new Promise((r) => setTimeout(r, ms)),
    polls = 80,
  } = {},
) {
  if (
    !/^srv-[a-z0-9]+$/.test(service || "") ||
    !/^[a-f0-9]{40}$/.test(commit || "") ||
    !token
  )
    throw Error("Missing or invalid Render configuration");
  const base = `https://api.render.com/v1/services/${service}/deploys`;
  async function call(url, body) {
    let response;
    try {
      response = await request(url, {
        method: body ? "POST" : "GET",
        redirect: "error",
        signal: AbortSignal.timeout(20000),
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        ...(body ? { body: JSON.stringify(body) } : {}),
      });
    } catch {
      throw Error(
        "Render request failed; inspect existing deploy before retrying",
      );
    }
    if (!response.ok) throw Error(`Render HTTP ${response.status}`);
    try {
      return await response.json();
    } catch {
      throw Error("Invalid Render response");
    }
  }
  const list = await call(base + "?limit=100");
  if (!Array.isArray(list)) throw Error("Invalid Render deploy list");
  const terminal = new Set([
    "live",
    "deactivated",
    "build_failed",
    "update_failed",
    "pre_deploy_failed",
    "canceled",
  ]);
  if (list.some((x) => !terminal.has(x.deploy?.status))) return "busy";
  const live = list.find((x) => x.deploy?.status === "live")?.deploy;
  if (live?.commit?.id !== commit)
    throw Error(
      "Pinned commit differs from live deploy; content rebuild refused",
    );
  const created = await call(base, {
    commitId: commit,
    clearCache: "do_not_clear",
  });
  if (!/^dep-[a-z0-9]+$/.test(created.id || ""))
    throw Error("Invalid deploy id; inspect Render before retrying");
  for (let n = 0; n < polls; n++) {
    const deploy = await call(base + "/" + created.id);
    if (deploy.status === "live") {
      if (deploy.commit?.id !== commit)
        throw Error("Unexpected deployed commit");
      return "published";
    }
    if (terminal.has(deploy.status))
      throw Error("Render deploy did not succeed");
    await sleep(15000);
  }
  throw Error("Render deploy timeout; next run must check outstanding deploy");
}
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  try {
    console.log(
      await publishContent({
        service: process.env.RENDER_STATIC_SERVICE_ID,
        commit: process.env.RENDER_CONTENT_COMMIT,
        token: process.env.RENDER_API_TOKEN,
      }),
    );
  } catch (e) {
    console.error(e.message);
    process.exitCode = 1;
  }
}
