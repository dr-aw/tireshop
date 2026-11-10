import { useEffect, type RefObject } from "react";

// One background transfer for the whole page, not one per gallery.
const queue: { url: string; owners: Map<symbol, () => boolean> }[] = [];
const pending = new Map<string, (typeof queue)[number]>();
const loaded = new Set<string>();
let active = false;
let scheduled = false;
let waitingForLoad = false;
function remember(url: string) {
  loaded.delete(url);
  loaded.add(url);
  if (loaded.size > 256) loaded.delete(loaded.values().next().value!);
}
function schedule() {
  if (typeof window === "undefined" || scheduled || active || !queue.length)
    return;
  if (document.readyState !== "complete") {
    if (!waitingForLoad) {
      waitingForLoad = true;
      window.addEventListener(
        "load",
        () => {
          waitingForLoad = false;
          schedule();
        },
        { once: true },
      );
    }
    return;
  }
  scheduled = true;
  // Let rendering and user input go first; requestIdleCallback isn't universal.
  if (typeof window.requestIdleCallback === "function")
    window.requestIdleCallback(pump, { timeout: 1500 });
  else window.setTimeout(pump, 100);
}
function pump() {
  scheduled = false;
  if (active) return;
  if (document.hidden) {
    document.addEventListener("visibilitychange", () => schedule(), {
      once: true,
    });
    return;
  }
  let task: (typeof queue)[number] | undefined;
  while ((task = queue.shift())) {
    if (
      loaded.has(task.url) ||
      ![...task.owners.values()].some((valid) => valid())
    ) {
      pending.delete(task.url);
      continue;
    }
    active = true;
    const current = task;
    const image = new Image();
    image.decoding = "async";
    image.fetchPriority = "low";
    let finished = false;
    const finish = (success: boolean) => {
      if (finished) return;
      finished = true;
      clearTimeout(timeout);
      image.onload = image.onerror = null;
      if (success) remember(current.url);
      pending.delete(current.url);
      active = false;
      schedule();
    };
    const timeout = window.setTimeout(() => {
      image.src = "";
      finish(false);
    }, 10000);
    image.onload = () => {
      void image
        .decode()
        .catch(() => {})
        .then(() => finish(true));
    };
    image.onerror = () => finish(false);
    image.src = current.url;
    return;
  }
}
function enqueue(urls: string[], valid: () => boolean) {
  const token = Symbol();
  for (const url of new Set(urls)) {
    if (loaded.has(url)) continue;
    let task = pending.get(url);
    if (!task) {
      task = { url, owners: new Map() };
      pending.set(url, task);
      queue.push(task);
    }
    task.owners.set(token, valid);
  }
  schedule();
  return () => {
    for (const [url, task] of pending) {
      task.owners.delete(token);
      if (!task.owners.size && queue.includes(task)) {
        queue.splice(queue.indexOf(task), 1);
        pending.delete(url);
      }
    }
  };
}
export function useAdjacentPreload(
  ref: RefObject<HTMLDivElement | null>,
  urls: string[],
  index: number,
  failed: boolean,
) {
  const signature = JSON.stringify(urls);
  useEffect(() => {
    const gallery = ref.current;
    const image = gallery?.querySelector("img");
    const sources: string[] = JSON.parse(signature);
    if (!gallery || !image || failed || sources.length < 2 || !sources[index])
      return;
    let visible = false,
      disposed = false;
    let cancel: (() => void) | undefined;
    const valid = () => !disposed && visible && gallery.isConnected;
    const update = () => {
      if (!valid() || !image.complete || !image.naturalWidth) return;
      remember(sources[index]);
      if (!cancel)
        cancel = enqueue(
          [
            sources[(index + 1) % sources.length],
            sources[(index + sources.length - 1) % sources.length],
          ].filter((url) => url !== sources[index]),
          valid,
        );
    };
    const observer = new IntersectionObserver((entries) => {
      visible = entries[0].isIntersecting;
      if (!visible) {
        cancel?.();
        cancel = undefined;
      } else update();
    });
    observer.observe(gallery);
    image.addEventListener("load", update);
    update();
    return () => {
      disposed = true;
      observer.disconnect();
      image.removeEventListener("load", update);
      cancel?.();
    };
  }, [ref, signature, index, failed]);
}
