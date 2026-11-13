/** Renders the empty state shown when no document is open. */
export function renderEmptyState(root: HTMLElement): void {
  const section = document.createElement("section");
  section.className = "empty";

  const title = document.createElement("h1");
  title.textContent = "zen";

  const hint = document.createElement("p");
  hint.textContent = "Open a markdown file to begin.";

  section.append(title, hint);
  root.replaceChildren(section);
}

/** Shows a document. `html` is rendered and sanitised by Go (core/render). */
export function renderDocument(
  root: HTMLElement,
  doc: { name: string; html: string; error?: string },
): void {
  const article = document.createElement("article");
  article.className = "document";

  if (doc.error) {
    const p = document.createElement("p");
    p.className = "error";
    p.textContent = `Could not open ${doc.name}: ${doc.error}`;
    article.append(p);
  } else {
    article.innerHTML = doc.html;
  }

  document.title = `${doc.name} — zen`;
  root.replaceChildren(article);
}
