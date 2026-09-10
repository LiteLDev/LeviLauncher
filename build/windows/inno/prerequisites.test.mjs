import { execFileSync, spawnSync } from "node:child_process";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import assert from "node:assert/strict";

// Compile and execute the production Pascal logic, replacing only OS effects.
// No real registry writes, prerequisite installers, or application installation.
const compiler = process.env.ISCC_PATH || "iscc";
const directory = await mkdtemp(join(tmpdir(), "levilauncher-prerequisites-"));
let source = await readFile(new URL("./prerequisites.iss", import.meta.url), "utf8");
for (const name of ["RegQueryStringValue", "RegQueryDWordValue", "ExtractTemporaryFile", "Exec"]) {
  source = source.replace(new RegExp(`\\b${name}\\b`, "g"), `Mock${name}`);
}
source = source.replace("WizardForm.PreparingLabel.Caption := StatusMessage;", "Log(StatusMessage);");
await writeFile(join(directory, "prerequisites.iss"), source);
const harness = String.raw`
#define VCRuntimeArch "x64"
#define VCRuntimeVersion "14.44.35211.0"
#define VCRuntimeFile "vc_redist.x64.exe"
[Setup]
AppName=Prerequisite regression tests
AppVersion=1.0
DefaultDirName={tmp}\unused
PrivilegesRequired=lowest
Uninstallable=no
CreateAppDir=no
OutputDir=.
OutputBaseFilename=tests
[CustomMessages]
InstallVCRuntime=Installing VC++
InstallWebView2=Installing WebView2
RuntimeInstallFailed=Failed %1: %2
RuntimeNotDetected=Missing %1
[Code]
var
  WVVersion, WVUserVersion, VCVersion, VC64Version: string;
  VCInstalled, VC64Installed: Cardinal;
  ExecCode, ExecCount, ExtractCount: Integer;
  ExecSucceeds, RegisterRuntime, ExtractionFails: Boolean;

function MockRegQueryStringValue(RootKey: Integer; const Key, Name: string; var Value: string): Boolean;
begin
  Value := '';
  if Pos('EdgeUpdate', Key) > 0 then
  begin
    if RootKey = HKLM32 then Value := WVVersion;
    if RootKey = HKCU then Value := WVUserVersion;
  end
  else if Pos('Runtimes\x64', Key) > 0 then
  begin
    if RootKey = HKLM32 then Value := VCVersion;
    if RootKey = HKLM64 then Value := VC64Version;
  end;
  Result := Value <> '';
end;

function MockRegQueryDWordValue(RootKey: Integer; const Key, Name: string; var Value: Cardinal): Boolean;
begin
  Value := 0;
  if RootKey = HKLM32 then Value := VCInstalled;
  if RootKey = HKLM64 then Value := VC64Installed;
  Result := True;
end;

procedure MockExtractTemporaryFile(const FileName: string);
begin
  ExtractCount := ExtractCount + 1;
  if ExtractionFails then RaiseException('extraction failed');
end;

function MockExec(const FileName, Parameters, WorkingDir: string; ShowCmd: Integer;
  Wait: TExecWait; var ResultCode: Integer): Boolean;
begin
  ExecCount := ExecCount + 1;
  Result := ExecSucceeds;
  ResultCode := ExecCode;
  if Result and RegisterRuntime then
  begin
    if Pos('vc_redist', FileName) > 0 then
    begin
      if Parameters <> '/install /quiet /norestart' then RaiseException('VC arguments');
      VCInstalled := 1;
      VCVersion := 'v{#VCRuntimeVersion}';
    end
    else
    begin
      if Parameters <> '/silent /install' then RaiseException('WebView2 arguments');
      WVVersion := '140.0.1.0';
    end;
  end;
end;

#include "prerequisites.iss"

procedure AssertTrue(Value: Boolean; const Message: string);
begin
  if not Value then RaiseException(Message);
end;

procedure Reset;
begin
  WVVersion := '';
  WVUserVersion := '';
  VCVersion := '';
  VC64Version := '';
  VCInstalled := 0;
  VC64Installed := 0;
  ExecCode := 0;
  ExecCount := 0;
  ExtractCount := 0;
  ExecSucceeds := True;
  RegisterRuntime := True;
  ExtractionFails := False;
  RuntimeRestartRequired := False;
end;

function InitializeSetup: Boolean;
var
  Error: string;
  Restart: Boolean;
begin
  Result := False;
  try
    Reset;
    AssertTrue(NeedsWebView2Runtime(), 'missing WV');
    WVUserVersion := '140.0.1.0';
    AssertTrue(NeedsWebView2Runtime(), 'user-only WV does not satisfy an all-users installer');
    WVVersion := '0.0.0.0';
    AssertTrue(NeedsWebView2Runtime(), 'zero WV must not skip installation');
    WVVersion := '   ';
    AssertTrue(NeedsWebView2Runtime(), 'blank WV');
    WVVersion := 'invalid';
    AssertTrue(NeedsWebView2Runtime(), 'malformed WV');
    WVVersion := '140.0.1.0';
    AssertTrue(not NeedsWebView2Runtime(), 'valid machine WV');
    AssertTrue(NeedsVCRuntime(), 'missing VC');
    VCInstalled := 1;
    VCVersion := 'v14.20.1.0';
    AssertTrue(NeedsVCRuntime(), 'old VC must be upgraded');
    VCVersion := 'v{#VCRuntimeVersion}';
    AssertTrue(not NeedsVCRuntime(), 'equal VC');
    VCVersion := 'v14.99.1.0';
    AssertTrue(not NeedsVCRuntime(), 'newer VC must not downgrade');
    VCInstalled := 0;
    AssertTrue(NeedsVCRuntime(), 'uninstalled VC registry residue');
    VC64Installed := 1;
    VC64Version := 'v{#VCRuntimeVersion}';
    AssertTrue(not NeedsVCRuntime(), '64-bit VC registry view');
    Restart := False;
    Error := PrepareToInstall(Restart);
    AssertTrue((Error = '') and (ExecCount = 0) and (ExtractCount = 0), 'skip installed runtimes');

    Reset;
    Error := PrepareToInstall(Restart);
    AssertTrue((Error = '') and (ExecCount = 2) and (ExtractCount = 2), 'install both missing runtimes');
    AssertTrue(not NeedRestart(), 'normal success without reboot');
    Error := PrepareToInstall(Restart);
    AssertTrue((Error = '') and (ExecCount = 2), 'retry must not reinstall');

    Reset;
    ExecCode := 1603;
    RegisterRuntime := False;
    Error := PrepareToInstall(Restart);
    AssertTrue((Pos('1603', Error) > 0) and (ExecCount = 1), 'VC failure blocks setup');
    ExecCode := 0;
    RegisterRuntime := True;
    Error := PrepareToInstall(Restart);
    AssertTrue((Error = '') and (ExecCount = 3), 'retry recovers after failure');

    Reset;
    VCInstalled := 1;
    VCVersion := 'v{#VCRuntimeVersion}';
    ExecCode := -2147219198;
    RegisterRuntime := False;
    Error := PrepareToInstall(Restart);
    AssertTrue((Pos('-2147219198', Error) > 0) and (ExecCount = 1), 'WV failure blocks setup');

    Reset;
    ExecSucceeds := False;
    ExecCode := 2;
    Error := PrepareToInstall(Restart);
    AssertTrue((Error <> '') and (ExecCount = 1), 'process launch failure');

    Reset;
    RegisterRuntime := False;
    Error := PrepareToInstall(Restart);
    AssertTrue((Pos('Missing', Error) > 0) and (ExecCount = 1), 'zero exit without registration is failure');

    Reset;
    VCInstalled := 1;
    VCVersion := 'v{#VCRuntimeVersion}';
    RegisterRuntime := False;
    Error := PrepareToInstall(Restart);
    AssertTrue(Pos('Missing WebView2', Error) > 0, 'WV post-install verification');

    Reset;
    ExecCode := 3010;
    Error := PrepareToInstall(Restart);
    AssertTrue((Error = '') and NeedRestart() and not Restart, '3010 schedules final reboot');

    Reset;
    ExecCode := 1641;
    Error := PrepareToInstall(Restart);
    AssertTrue((Error = '') and NeedRestart(), '1641 requests reboot');

    Reset;
    ExecCode := 3010;
    RegisterRuntime := False;
    Error := PrepareToInstall(Restart);
    AssertTrue((Error <> '') and Restart, 'pending reboot propagates on early failure');

    Reset;
    ExtractionFails := True;
    Error := PrepareToInstall(Restart);
    AssertTrue((Pos('extraction failed', Error) > 0) and (ExecCount = 0), 'extraction failure blocks setup');
    SaveStringToFile(ExpandConstant('{src}\result.txt'), 'PASS', False);
  except
    SaveStringToFile(ExpandConstant('{src}\result.txt'), 'FAIL: ' + GetExceptionMessage, False);
  end;
end;
`;
await writeFile(join(directory, "tests.iss"), harness);
execFileSync(compiler, ["/Qp", join(directory, "tests.iss")], { stdio: "inherit", windowsHide: true });
const execution = spawnSync(join(directory, "tests.exe"), ["/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"], {
  windowsHide: true, timeout: 60_000,
});
if (execution.error) throw execution.error;
assert.equal(await readFile(join(directory, "result.txt"), "utf8"), "PASS");
console.log(`Prerequisite Pascal regression tests passed (${directory})`);

// Guard both packaging entry points and the actual embedded payload declarations.
const installer = await readFile(new URL("./project.iss", import.meta.url), "utf8");
assert.match(installer, /Source: "\{#VCRuntimeFile\}"; Flags: dontcopy/);
assert.match(installer, /Source: "MicrosoftEdgeWebview2Setup.exe"; Flags: dontcopy/);
assert.match(installer, /#include "prerequisites.iss"/);
for (const name of ["Taskfile.yml", "Taskfile.inno.yml"]) {
  const content = await readFile(new URL(`../${name}`, import.meta.url), "utf8");
  assert.match(content, /download-vcredist\.mjs.*\{\{\.ARCH\}\}/);
  assert.match(content, /generate webview2bootstrapper/);
}
console.log(`Packaging integration checks passed (${fileURLToPath(import.meta.url)})`);
