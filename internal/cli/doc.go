// Package cli is LeviLauncher's command line: LeviLauncher.exe <noun> <verb>
// [arguments] [flags]. main calls Wants to tell a command line from the
// launcher window's own arguments, and Run to execute it. The CLI never
// creates a Wails application, never loads WebView2 and never takes the
// single-instance lock; it prepares storage and settings (storage.Prepare,
// apppath.EnsureDirs, config.Load) and runs the command in-process.
//
// # Adding a command
//
// One file per command or small group of commands, named cmd_<noun>_<verb>.go
// (cmd_instance_create.go) or cmd_<noun>.go. The file registers its commands
// from init() and touches no other file of the package:
//
//	package cli
//
//	func init() {
//		Register(&Command{
//			Path:     "world list",           // "<noun> <verb>", or one word for a top-level command
//			Summary:  "list the worlds of an instance and player",
//			Args:     "[<filter>]",
//			Examples: []string{"world list", "world list -i Survival --json"},
//			Flags:    []*Flag{BoolFlag("size", "", "also compute folder sizes")},
//			MaxArgs:  1,
//			Run:      runWorldList,
//		})
//		RegisterBackendErrors(map[string]BackendError{
//			"ERR_WORLD_LOCKED": {Exit: ExitConflict, Message: "the world is open in the game", Hint: "LeviLauncher.exe stop"},
//		})
//	}
//
//	type worldItem struct {
//		Name string `json:"name"`
//		ID   string `json:"id"`
//	}
//
//	func runWorldList(c *Ctx) error {
//		inst, err := c.ResolveInstance("")   // -i, env, default, only instance
//		if err != nil {
//			return err
//		}
//		player, err := c.ResolvePlayer(inst) // -p, env, only profile, signed-in account
//		if err != nil {
//			return err
//		}
//		items := []worldItem{}               // never nil: --json prints []
//		// ... fill items from the backend (contentmgr, mcservice, ...)
//		ids := make([]string, len(items))
//		for i, it := range items {
//			ids[i] = it.Name
//		}
//		return c.Render(items, ids, func(w io.Writer) error {
//			rows := make([][]string, len(items))
//			for i, it := range items {
//				rows[i] = []string{it.Name, it.ID}
//			}
//			return Table(w, []string{"NAME", "ID"}, rows)
//		})
//	}
//
// The standard nouns (instance, mod, loader, world, pack, screenshot,
// server, player, backup, installer, curseforge, process, account, config,
// cache) and the top-level aliases install, stop and ps are declared in
// nouns.go: a verb only needs Register. A noun with no verb yet is hidden.
// Two files may add verbs to the same noun. "ls" and "rm" are added to
// every "list" and "remove" verb automatically; other aliases go in
// Command.Aliases ("play" for "launch"). Top-level commands set Group to
// appear in the overview help. Registration problems (duplicate words,
// unknown nouns, flags clashing with globals) panic on first use and fail
// TestRegistryIsConsistent, which also resolves every example.
//
// # Conventions
//
//   - stdout carries only the result, through Ctx.Render (or Ctx.JSON,
//     Table, KV, Ctx.Printf). Everything else goes to stderr: Ctx.Notice
//     (suppressed by -q and --json), Ctx.Warn, Ctx.Verbosef (-v only),
//     progress and errors.
//   - --json prints one document: an array for lists, an object otherwise,
//     camelCase keys from json tags, RFC 3339 times, sizes in bytes. -q
//     prints identifiers only.
//   - Long operations block until the real outcome is known, pass
//     c.Context() (cancelled on Ctrl-C) to every blocking call, and report
//     progress with c.Progress() (Phase, Bytes, Count, Done). Backend events
//     (downloads, extraction, LIP tasks, backups, self-update, runtime
//     installers, loader migration) are rendered automatically: Run installs
//     the progress sink as eventsink's default. Progress.Listen observes raw
//     events (done/error).
//   - Errors are returned, never printed: an *Error from Errorf, UsageError,
//     NotFoundError, ConflictError, or Backend(resultString) for backend
//     "ERR_*" strings, or FromError(err) for Go errors. Register the
//     messages, hints and exit codes of your backend codes with
//     RegisterBackendErrors from your own file; a code other command groups
//     also get with another meaning (ERR_WRITE_FILE, ERR_INVALID_PATH,
//     ERR_NAME_*, ...) goes in RegisterBackendErrorsFor with your nouns or
//     command paths, and errors.go holds its neutral wording. ExitCode(n) ends with n and
//     prints nothing (batch summaries: ExitPartial when some items failed,
//     ExitFailure when all failed).
//   - Destructive or displacing actions call c.Confirm("Remove X?") (or
//     ConfirmWith for an action answered by --overwrite-like flags) before
//     acting. It returns nil on --yes or "y", exit 10 when declined and exit 2
//     when it cannot prompt.
//   - Names match exactly, ignoring case; never by prefix. LookupInstance,
//     ResolveInstance and ResolvePlayer implement the rules and the
//     implicit-choice echo. Idempotent requests exit 0.
//   - Messages are plain English sentences, lower case, without a final
//     period; hints are runnable commands starting with ProgramName.
//
// # Backend helpers for commands
//
//   - eventsink: backend events go to the context's sink, else the CLI's
//     progress sink. eventsink.With(ctx, sink) routes one operation's events.
//   - launch.Watch / launch.WaitGameExit / launch.WaitPIDExit /
//     launch.GameProcessIDs: launch with versionlaunch.Launcher.Launch(ctx,
//     ...) using Watch's context, then wait for the window, a failure or a
//     timeout.
//   - downloader.Download, msixvc.DownloadSync, msixvc.DownloadNamedSync:
//     blocking, resumable downloads; Ctrl-C keeps the partial file.
//   - update.CheckUpdateErr, update.Apply: self-update without restarting or
//     exiting; GUIRunning reports a running launcher window.
//   - SignIn, SignOut, RestoreAccount: the Microsoft account (winhost owns
//     the account picker).
//   - config.GetCurrentInstance / config.SetCurrentInstance: the default
//     instance ("use"). config.Update re-reads config.json first, so the CLI
//     and a running window do not overwrite each other's settings.
//
// Do not construct internal/app services, call application.Get(), or write
// to os.Stdout/os.Stderr directly from a command.
package cli
