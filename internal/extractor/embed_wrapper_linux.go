//go:build linux

package extractor

import _ "embed"

//go:embed launcher_core_cli.exe
var embeddedLauncherCoreCLI []byte