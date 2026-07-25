import { Application, Browser, Events, Window } from "@wailsio/runtime";

export function EventsOn<T>(eventName: string, callback: (data: T) => void): () => void {
  try {
    const unsub = Events.On(eventName, (event) => {
      callback(event.data as T);
    });
    return unsub || (() => {});
  } catch (err) {
    console.warn("[Wails Runtime] EventsOn not supported in standalone browser:", err);
    return () => {};
  }
}

export function Quit(): Promise<void> {
  try {
    return Application.Quit();
  } catch {
    return Promise.resolve();
  }
}

export function WindowMinimise(): Promise<void> {
  try {
    return Window.Minimise();
  } catch {
    return Promise.resolve();
  }
}

export function OpenURL(url: string): Promise<void> {
  try {
    return Browser.OpenURL(url);
  } catch {
    window.open(url, "_blank");
    return Promise.resolve();
  }
}

export async function WindowToggleMaximise(): Promise<void> {
  try {
    if (await Window.IsMaximised()) {
      await Window.Restore();
      return;
    }
    await Window.Maximise();
  } catch {
    // ignore in browser
  }
}
