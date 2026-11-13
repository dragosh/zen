import { Documents } from "../bindings/github.com/dragosh/zen/internal/app";
import { renderDocument, renderEmptyState } from "./shell";

const root = document.getElementById("app")!;
renderEmptyState(root);

Documents.Startup()
  .then((docs) => {
    if (docs && docs.length > 0) renderDocument(root, docs[0]);
  })
  .catch((err) => console.error("zen: loading documents failed", err));
