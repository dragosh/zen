import { afterEach, beforeEach, expect, test } from "vitest";
import { installImageZoom, ZOOM_MAX, ZOOM_MIN, type ImageZoom } from "./zoom";

let root: HTMLElement;
let zoom: ImageZoom;

/** An image with natural size w×h (happy-dom loads nothing, so the size is stubbed). */
function image(html: string, w = 2000, h = 1000): HTMLImageElement {
  root.innerHTML = html;
  const img = root.querySelector("img")!;
  Object.defineProperty(img, "naturalWidth", { value: w, configurable: true });
  Object.defineProperty(img, "naturalHeight", { value: h, configurable: true });
  Object.defineProperty(img, "complete", { value: true, configurable: true });
  return img;
}
const click = (n: Element, init: MouseEventInit = {}) =>
  n.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true, button: 0, ...init }));
const key = (k: string) => document.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
const view = () => document.querySelector<HTMLElement>(".zen-zoom");
const shown = () => view()!.querySelector<HTMLImageElement>(".zen-zoom-img")!;
const scale = () => view()!.querySelector(".zen-zoom-scale")!.textContent;
const width = () => parseFloat(shown().style.width);

beforeEach(() => {
  document.body.replaceChildren();
  Object.defineProperty(window, "innerWidth", { value: 1064, configurable: true });
  Object.defineProperty(window, "innerHeight", { value: 864, configurable: true });
  root = document.body.appendChild(document.createElement("article"));
  root.className = "zen-page";
  zoom = installImageZoom(root);
});
afterEach(() => zoom.destroy());

test("a click on a page image opens it over the page, fitted to the window", () => {
  const img = image('<p><img src="shot.png" alt="A screenshot"></p>'); // 2000×1000 in a 1000×800 room
  click(img);
  expect(zoom.isOpen).toBe(true);
  expect(view()!.getAttribute("role")).toBe("dialog");
  expect(view()!.getAttribute("aria-label")).toBe("Image: A screenshot");
  expect(shown().src).toMatch(/\/shot\.png$/);
  expect(shown().alt).toBe("A screenshot");
  expect(width()).toBe(1000); // (1064 - 2·32) / 2000 = 50%
  expect(scale()).toBe("50%");
});

test("the fit never enlarges an image past 100%", () => {
  click(image('<img src="small.png">', 400, 300));
  expect(scale()).toBe("100%");
  expect(width()).toBe(400);
});

test.each([
  ["in a link", '<a href="page.md"><img src="a.png"></a>'],
  ["in a button", '<button><img src="a.png"></button>'],
  ["in a plugin", '<div data-zen-plugin="x"><img src="a.png"></div>'],
  ["marked nozoom", '<img src="a.png" data-zen-nozoom>'],
])("an image %s is not zoomed", (_name, html) => {
  click(image(html));
  expect(zoom.isOpen).toBe(false);
});

test("broken, unloaded and small images are not zoomed", () => {
  click(image('<img src="broken.png">', 0, 0));
  expect(zoom.isOpen).toBe(false);
  click(image('<img src="badge.svg">', 40, 20));
  expect(zoom.isOpen).toBe(false);
  const img = image('<img src="slow.png">');
  Object.defineProperty(img, "complete", { value: false });
  click(img);
  expect(zoom.isOpen).toBe(false);
});

test("modifier clicks, other buttons and handled clicks are left alone", () => {
  const img = image('<img src="a.png">');
  for (const init of [{ metaKey: true }, { ctrlKey: true }, { shiftKey: true }, { altKey: true }, { button: 1 }]) {
    click(img, init);
    expect(zoom.isOpen).toBe(false);
  }
  root.addEventListener("click", (e) => e.preventDefault(), { capture: true, once: true });
  click(img);
  expect(zoom.isOpen).toBe(false);
});

test("a click on the image switches between fit and 100% for a large image", () => {
  click(image('<img src="a.png">'));
  click(shown());
  expect(scale()).toBe("100%");
  expect(width()).toBe(2000);
  expect(zoom.isOpen).toBe(true); // the click on the image does not close the view
  click(shown());
  expect(scale()).toBe("50%");
});

test("a click on a small image switches between fit and twice the fit", () => {
  click(image('<img src="a.png">', 400, 300));
  click(shown());
  expect(scale()).toBe("200%");
  click(shown());
  expect(scale()).toBe("100%");
});

test("keys and a pinch change the scale within the limits", () => {
  click(image('<img src="a.png">'));
  key("+");
  expect(scale()).toBe("63%"); // 50% × 1.25
  key("-");
  expect(scale()).toBe("50%");
  key("1");
  expect(scale()).toBe("100%");
  key("0");
  expect(scale()).toBe("50%");
  for (let i = 0; i < 40; i++) key("+");
  expect(scale()).toBe(`${ZOOM_MAX * 100}%`);
  for (let i = 0; i < 60; i++) key("-");
  expect(scale()).toBe(`${ZOOM_MIN * 100}%`);
  key("0");
  const pinch = new WheelEvent("wheel", { deltaY: -50, bubbles: true, cancelable: true });
  Object.defineProperty(pinch, "ctrlKey", { value: true }); // happy-dom's WheelEvent has no modifier keys
  view()!.dispatchEvent(pinch);
  expect(pinch.defaultPrevented).toBe(true);
  expect(width()).toBeGreaterThan(1000); // pinch out zooms in
});

test("the bar zooms, fits and closes", () => {
  click(image('<img src="a.png">'));
  const btn = (label: string) => view()!.querySelector<HTMLButtonElement>(`button[aria-label^="${label}"]`)!;
  click(btn("Zoom in"));
  expect(scale()).toBe("63%");
  click(btn("Fit"));
  expect(scale()).toBe("50%");
  click(btn("Zoom out"));
  expect(scale()).toBe("40%");
  click(btn("Close"));
  expect(view()).toBeNull();
});

test("Esc or a click outside the image closes it, and focus goes back", () => {
  const before = document.body.appendChild(document.createElement("button"));
  before.focus();
  click(image('<img src="a.png">'));
  expect(document.activeElement?.getAttribute("aria-label")).toBe("Close (Esc)");
  key("Escape");
  expect(view()).toBeNull();
  expect(document.activeElement).toBe(before);

  click(root.querySelector("img")!);
  click(view()!.querySelector(".zen-zoom-stage")!);
  expect(zoom.isOpen).toBe(false);
});

test("keys do nothing once it is closed; one view at a time", () => {
  const img = image('<img src="a.png">');
  click(img);
  click(img); // the page image is under the view in real life; opening again replaces the view
  expect(document.querySelectorAll(".zen-zoom")).toHaveLength(1);
  zoom.close();
  const e = new KeyboardEvent("keydown", { key: "+", cancelable: true });
  document.dispatchEvent(e);
  expect(e.defaultPrevented).toBe(false);
});

test("destroy closes the view and stops listening", () => {
  const img = image('<img src="a.png">');
  click(img);
  zoom.destroy();
  expect(view()).toBeNull();
  click(img);
  expect(view()).toBeNull();
});
