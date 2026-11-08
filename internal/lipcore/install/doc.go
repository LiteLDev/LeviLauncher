// Package install ports lip's package installer
// (src/Lip.Core/Mcmod/Install/McmodInstaller.cs, PackagePlan.cs,
// FileOwners.cs, WorkspaceView.cs), the workspace layout
// (src/Lip.Core/Mcmod/WorkspaceLayout.cs) and the hook command runner
// (src/Lip.Core/Infrastructure/CommandRunner.cs), from v0.38.0-beta.1 with
// the fixes of lip main (protocol/workspace.md §2, §4, §6, §7).
//
// An operation (McmodService.Execute, wave 3) first plans every change
// against one View of the workspace, so that every check and every fetch
// happens before anything changes:
//
//	layout, err := install.PlanLayout(root)        // §2: mods/ is created when missing
//	view := install.NewView(layout)
//	plan, err := inst.PlanUninstall(ctx, view, opts, locked, entry)
//	plan, err := inst.PlanInstall(ctx, view, opts, pkg)
//
// checks the layout when a plan may change the workspace
// (Plan.MayChange; a mods that is a link fails), then applies the plans
// through one txn.Transaction (whose Options.Reserved holds the WrittenPaths
// of every plan) and commits it, or rolls it back on the first error:
//
//	install.CheckLayout(layout)                    // §2: mods/ is a real directory
//	install.ApplyLayout(tx, layout)
//	owners := inst.Owners(root, installed)
//	files, err := inst.Apply(ctx, tx, plan, owners) // for each plan, in order
//
// Whether a file matching preserve is kept, and what uninstalling deletes,
// are decided when each step runs, after the hook commands before it; each
// write is checked against the write rules and the owners as they are then
// (FileOwners). On Windows paths are checked by their long names, so that an
// 8.3 short name cannot reach .lip or a file of another package.
//
// Planning fails with *Error (McmodInstallException) and changes nothing;
// Apply fails with *Error too, or with the context's error when cancelled.
// The layout fails with *LayoutError (WorkspaceLayoutException). Messages are
// lip's.
//
// Like lip, a dry run reports what it would write, keep and delete through
// Installer.Reporter (Info) while planning; the operation-level messages
// ("Dry run: would install …") are the service's.
//
// Measured progress: Apply reports a report.StagePlace bar over the bytes of
// the files it writes (or the files, when a size is unknown), a
// report.StageRemove bar over the files and directories it deletes, and a
// report.StageScripts bar per hook, as the 0.37 engine/workspace did.
//
// Paths are compared ignoring case on Windows and ordinally elsewhere
// (WorkspaceLayout.PathComparer). On Windows, a purge pattern or recorded
// path with a name that ends with '.' or a space, or contains ':', matches
// nothing: Windows cannot hold such a name, and Path.GetFullPath would turn
// it into another path (e.g. "mods./" into mods).
//
// The process handling of DefaultCommandRunner (process tree, job object,
// output pipes, CliWrap's argument escaping) is carried over from the 0.37
// engine/workspace, which this package replaces.
package install
