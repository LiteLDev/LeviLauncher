package cli

// The standard nouns and top-level aliases. They are declared here, once,
// so that command files only add verbs: a noun without any registered verb
// is hidden from help and dispatch, and an alias whose target is not
// registered does nothing. Do not add verbs here.

func init() {
	for _, n := range []*Noun{
		{Name: "instance", Group: GroupManage, Default: "show",
			Summary: "installed Minecraft instances: list, inspect, create, change and remove them"},
		{Name: "mod", Group: GroupManage, Summary: "mods of an instance: local, LIP catalogue and imported"},
		{Name: "loader", Group: GroupManage, Note: "(LeviLamina)", Summary: "the LeviLamina mod loader of an instance"},
		{Name: "world", Group: GroupManage, Summary: "worlds of an instance and player"},
		{Name: "pack", Group: GroupManage, Summary: "resource, behavior and skin packs of an instance"},
		{Name: "screenshot", Group: GroupManage, Summary: "screenshots of an instance and player"},
		{Name: "server", Group: GroupManage, Summary: "the server list of an instance and player"},
		{Name: "player", Group: GroupManage, Summary: "player profiles of a GDK instance"},
		{Name: "backup", Group: GroupManage, Note: "(preview)", Summary: "instance backups"},
		{Name: "installer", Group: GroupFind, Note: "(Minecraft builds)", Summary: "downloadable Minecraft builds and downloaded installers"},
		{Name: "curseforge", Aliases: []string{"cf"}, Group: GroupFind, Summary: "CurseForge content for Minecraft Bedrock"},
		{Name: "process", Group: GroupSystem, Note: "(alias: ps)", Summary: "running Minecraft processes"},
		{Name: "account", Group: GroupSystem, Summary: "the Microsoft account the launcher uses"},
		{Name: "config", Group: GroupSystem, Summary: "launcher settings"},
		{Name: "cache", Group: GroupSystem, Summary: "launcher caches"},
	} {
		RegisterNoun(n)
	}
	RegisterTopAlias("install", "instance create")
	RegisterTopAlias("stop", "process stop")
	RegisterTopAlias("ps", "process list")
}
