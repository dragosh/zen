// Copy text to the clipboard: navigator.clipboard when the webview allows it,
// otherwise a hidden textarea and execCommand("copy").

/** Resolves to whether the text was copied. Never rejects. */
export async function copyText(doc: Document, text: string): Promise<boolean> {
  try {
    const clip = globalThis.navigator?.clipboard;
    if (clip?.writeText) {
      await clip.writeText(text);
      return true;
    }
  } catch {
    // denied or not secure: fall through to the textarea
  }
  const area = doc.createElement("textarea");
  area.value = text;
  area.setAttribute("readonly", "");
  area.style.position = "fixed";
  area.style.opacity = "0";
  doc.body.append(area);
  try {
    area.select();
    return doc.execCommand("copy");
  } catch {
    return false;
  } finally {
    area.remove();
  }
}
