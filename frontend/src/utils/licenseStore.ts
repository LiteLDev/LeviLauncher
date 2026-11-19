// The signed-in account's game licenses as the account menu last checked
// them, shared with pages that warn before an action the license blocks.

import { useSyncExternalStore } from "react";
import type { LicenseState } from "@/utils/troubleshoot";

export type LicenseSnapshot = {
  // false until an account has been resolved; signed-out sets it true with
  // an empty xuid.
  known: boolean;
  xuid: string;
  // With no xuid: an account is selected but Xbox Live could not be reached
  // to confirm it, so the user is not actually signed out.
  unreachable?: boolean;
  release: LicenseState;
  preview: LicenseState;
};

let snapshot: LicenseSnapshot = { known: false, xuid: "", release: "checking", preview: "checking" };
const listeners = new Set<() => void>();

export const publishLicenses = (next: LicenseSnapshot) => {
  if (
    next.known === snapshot.known &&
    next.xuid === snapshot.xuid &&
    Boolean(next.unreachable) === Boolean(snapshot.unreachable) &&
    next.release === snapshot.release &&
    next.preview === snapshot.preview
  ) {
    return;
  }
  snapshot = next;
  listeners.forEach((listener) => listener());
};

const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => listeners.delete(listener);
};

export const useLicenseSnapshot = (): LicenseSnapshot =>
  useSyncExternalStore(subscribe, () => snapshot, () => snapshot);
