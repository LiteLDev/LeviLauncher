import { useEffect, useState } from "react";
import { IsElevated } from "bindings/github.com/liteldev/LeviLauncher/internal/app/userservice";

// Elevation is fixed for the lifetime of the launcher process. Share the request
// between navigation layouts and the account panel.
let elevationRequest: Promise<boolean> | undefined;

export function useProcessElevation() {
  const [elevated, setElevated] = useState(false);
  useEffect(() => {
    let active = true;
    elevationRequest ??= IsElevated().catch((error: unknown) => {
      console.error("Failed to read process elevation", error);
      return false;
    });
    void elevationRequest.then((value) => {
      if (active) setElevated(value);
    });
    return () => { active = false; };
  }, []);
  return elevated;
}
