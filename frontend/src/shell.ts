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
