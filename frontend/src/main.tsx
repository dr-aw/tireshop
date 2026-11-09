import { createRoot, hydrateRoot } from "react-dom/client";
import "./style.css";
import "./site.css";
import { isStore, type PageData } from "./page-data";
const root = document.getElementById("root")!;
const embedded = document.getElementById("page-data");
const data: PageData = embedded
  ? JSON.parse(embedded.textContent!)
  : { path: location.pathname, products: [], total: 0 };
if (/^\/administrator\/?$/.test(location.pathname)) {
  import("./administrator-main");
} else if (!isStore(data.path)) {
  import("./information-main");
} else {
  const { App } = await import("./store");
  const page = <App data={data} />;
  if (embedded) hydrateRoot(root, page);
  else createRoot(root).render(page);
}
