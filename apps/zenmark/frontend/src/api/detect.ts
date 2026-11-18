// Inside the Zenmark app or in a browser? (zenmark-org/zenmd#1)
//
// On macOS, Wails registers the WKWebView message handler "external" in the web
// view's configuration, before any page script runs, but only adds
// window._wails (and window._wails.invoke) when the page has finished loading
// (the WebViewDidFinishNavigation hook). Looking for window._wails at start-up
// therefore raced the page load, and a lost race showed the browser fake.

type Win = (Window & typeof globalThis) | Record<string, any>;

const hasInvoke = (w: Win): boolean => typeof (w as any)._wails?.invoke === "function";

/** True inside the Wails app (the external message handler or Wails itself is there); false in a browser. */
export function inApp(w: Win | undefined = typeof window === "undefined" ? undefined : window): boolean {
  if (!w) return false;
  const x = w as any;
  return "_wails" in x || Boolean(x.webkit?.messageHandlers?.external);
}

/** Resolves true once Wails can take calls (window._wails.invoke), false after timeout ms. */
export function wailsReady(w: Win & Partial<EventTarget> = window, timeout = 5000): Promise<boolean> {
  if (hasInvoke(w)) return Promise.resolve(true);
  return new Promise((resolve) => {
    const started = Date.now();
    const finish = (ok: boolean) => {
      clearInterval(poll);
      w.removeEventListener?.("wails:runtime-config-ready", check);
      resolve(ok);
    };
    function check(): void {
      if (hasInvoke(w)) finish(true);
      else if (Date.now() - started >= timeout) finish(false);
    }
    w.addEventListener?.("wails:runtime-config-ready", check);
    const poll = setInterval(check, 20);
  });
}
