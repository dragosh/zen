// .zen/plugins/progress/progress.js — :::progress{id=...} checklists that remember.
// No backend: zen renders the container's task list, this script makes it clickable.
export default {
  async mount(el, ctx) {
    const boxes = Array.from(el.querySelectorAll('input[type="checkbox"]'));
    const count = document.createElement("p");
    count.className = "progress-count";
    count.setAttribute("role", "status");
    if (boxes.length === 0) {
      count.textContent = "No tasks in this list.";
      el.append(count);
      return () => count.remove();
    }

    // One stored value per list: the texts of the ticked items.
    const key = (ctx.attrs.id || "default").replace(/[^A-Za-z0-9._-]/g, "-").slice(0, 64);
    const label = (box) => box.parentElement.textContent.trim();
    const draw = () => {
      const done = boxes.filter((b) => b.checked).length;
      count.textContent = `${done} of ${boxes.length} done`;
    };

    const saved = await ctx.storage.get(key);
    if (Array.isArray(saved)) for (const b of boxes) b.checked = saved.includes(label(b));

    const onChange = async () => {
      draw();
      try {
        await ctx.storage.set(key, boxes.filter((b) => b.checked).map(label));
      } catch (err) {
        count.textContent = `Could not save: ${err.message}`;
      }
    };
    for (const b of boxes) b.disabled = false;
    el.addEventListener("change", onChange);
    draw();
    el.append(count);

    return () => {
      el.removeEventListener("change", onChange);
      for (const b of boxes) b.disabled = true;
      count.remove();
    };
  },
};
