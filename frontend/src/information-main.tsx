import { createRoot, hydrateRoot } from "react-dom/client";
import { Information } from "./information";
import "./style.css";
import "./site.css";
const root = document.getElementById("root")!;
const data = document.getElementById("page-data");
const page = (
  <Information
    path={data ? JSON.parse(data.textContent!).path : location.pathname}
  />
);
if (data) hydrateRoot(root, page);
else createRoot(root).render(page);
