import { Application, Events, Window } from "@wailsio/runtime";

export function EventsOn<T>(eventName: string, callback: (data: T) => void): () => void {
  return Events.On(eventName, (event) => {
    callback(event.data as T);
  });
}

export function Quit(): Promise<void> {
  return Application.Quit();
}

export function WindowMinimise(): Promise<void> {
  return Window.Minimise();
}

export async function WindowToggleMaximise(): Promise<void> {
  if (await Window.IsMaximised()) {
    await Window.Restore();
    return;
  }
  await Window.Maximise();
}
