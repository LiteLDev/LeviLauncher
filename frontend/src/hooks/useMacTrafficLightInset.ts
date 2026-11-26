import { useEffect, useState } from "react";
import { Events, System, Window } from "@wailsio/runtime";

export function useMacTrafficLightInset() {
  const isMac = System.IsMac();
  const [fullscreen, setFullscreen] = useState(false);

  useEffect(() => {
    if (!isMac) return;
    let active = true;
    let receivedEvent = false;
    const { WindowFullscreen, WindowUnFullscreen } = Events.Types.Common;
    const update = (value: boolean) => {
      receivedEvent = true;
      setFullscreen(value);
    };
    const offEnter = Events.On(WindowFullscreen, () => update(true));
    const offExit = Events.On(WindowUnFullscreen, () => update(false));
    void Window.IsFullscreen().then((value) => {
      if (active && !receivedEvent) setFullscreen(value);
    }).catch(() => {});
    return () => {
      active = false;
      offEnter();
      offExit();
    };
  }, [isMac]);

  return isMac && !fullscreen;
}
