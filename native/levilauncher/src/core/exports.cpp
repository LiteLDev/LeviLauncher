// SPDX-License-Identifier: GPL-3.0-only
// Stable anchor imported by the launcher's PE import-table patch.
// Resolving this symbol loads the DLL; callers need not invoke it.
extern "C" __declspec(dllexport) void LeviLauncherEntry() noexcept {}
