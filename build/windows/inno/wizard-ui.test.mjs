import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

// Use Inno's actual controls and font metrics. No app/registry changes, runtime
// installers, modal dialogs, or changes to the user's Windows display settings.
const installer = await readFile(new URL("./project.iss", import.meta.url), "utf8");
const prerequisites = await readFile(new URL("./prerequisites.iss", import.meta.url), "utf8");
const compiler = process.env.ISCC_PATH || "iscc";
const directory = await mkdtemp(join(tmpdir(), "levilauncher-wizard-ui-"));
const languages = [...installer.matchAll(/^Name: "([a-z]+)"; MessagesFile:/gm)].map((m) => m[1]);
const section = (name) => {
  const match = installer.match(new RegExp(`^\\[${name}\\]\\r?\\n([\\s\\S]*?)(?=^\\[|$(?![\\s\\S]))`, "m"));
  assert.ok(match, `missing ${name} section`);
  return match[1];
};
const functionSource = (name) => {
  const match = installer.match(new RegExp(`^function ${name}\\b[\\s\\S]*?^end;`, "m"));
  assert.ok(match, `missing ${name} function`);
  return match[0];
};
let dialog = installer.slice(installer.indexOf("function AddDialogLabel("), installer.indexOf("function InitializeUninstall("));
assert.ok(dialog.includes("if OptionsForm.ShowModal <> mrOk then"));
dialog = dialog.replace(/\bDirExists\b/g, "FixtureDirExists").replace(
  "if OptionsForm.ShowModal <> mrOk then",
  "if ValidateDialog(OptionsForm, OkButton, CancelButton, InstallersCheck, VersionsCheck, BackupsCheck) <> mrOk then",
);
const harness = String.raw`
#define AppName "LeviLauncher"
[Setup]
AppName=LeviLauncher UI layout tests
AppVersion=1.0
DefaultDirName={tmp}\unused
PrivilegesRequired=lowest
Uninstallable=no
CreateAppDir=no
DisableWelcomePage=no
OutputDir=.
OutputBaseFilename=tests
[Languages]
${section("Languages")}
[LangOptions]
${section("LangOptions")}
DialogFontSize={#TestFontSize}
[CustomMessages]
${section("CustomMessages")}
[Code]
const
  DialogMargin = 16;
  DangerColor = clMaroon;
var
  RemoveInstallers, RemoveVersions, RemoveBackups: Boolean;
  CurrentInnoInstallDir, LegacyNsisInstallDir: string;
  FixtureTargets, FixtureSelected: Integer;
  FixtureCancel: Boolean;
  Metrics: string;
  PreparingProgress: TNewProgressBar;
  PreparingLabelHeight: Integer;

${prerequisites.slice(prerequisites.indexOf("procedure InitializePreparingProgress;"), prerequisites.indexOf("function HasWebView2Runtime("))}

procedure AssertTrue(Value: Boolean; const Message: string);
begin
  if not Value then RaiseException(Message);
end;

function FixtureDirExists(const Path: string): Boolean;
begin
  Result :=
    ((ExtractFileName(Path) = 'installers') and ((FixtureTargets and 1) <> 0)) or
    ((ExtractFileName(Path) = 'versions') and ((FixtureTargets and 2) <> 0)) or
    ((ExtractFileName(Path) = 'backups') and ((FixtureTargets and 4) <> 0));
end;

function ValidateDialog(Form: TSetupForm; OKButton, CancelButton: TNewButton;
  Installers, Versions, Backups: TNewCheckBox): Integer;
var
  I, J: Integer;
  A, B: TControl;
  Bitmap: TBitmap;
begin
  Bitmap := TBitmap.Create;
  try
    AssertTrue(Form.ClientWidth = ScaleX(460), 'Dialog must scale once, not twice');
    for I := 0 to Form.ControlCount - 1 do
    begin
      A := Form.Controls[I];
      AssertTrue((A.Left >= 0) and (A.Top >= 0) and
        (A.Left + A.Width <= Form.ClientWidth) and
        (A.Top + A.Height <= Form.ClientHeight), 'Control outside dialog');
      if A is TNewCheckBox then
      begin
        Bitmap.Canvas.Font.Assign(TNewCheckBox(A).Font);
        AssertTrue(Bitmap.Canvas.TextWidth(TNewCheckBox(A).Caption) + ScaleX(20) <= A.Width,
          'Checkbox text clipped: ' + TNewCheckBox(A).Caption);
        AssertTrue(not TNewCheckBox(A).Checked, 'Cleanup must be opt-in');
      end;
      if A is TNewButton then
      begin
        Bitmap.Canvas.Font.Assign(TNewButton(A).Font);
        AssertTrue(Bitmap.Canvas.TextWidth(TNewButton(A).Caption) + ScaleX(12) <= A.Width,
          'Button text clipped: ' + TNewButton(A).Caption);
      end;
      for J := I + 1 to Form.ControlCount - 1 do
      begin
        B := Form.Controls[J];
        AssertTrue(not ((A.Left < B.Left + B.Width) and (B.Left < A.Left + A.Width) and
          (A.Top < B.Top + B.Height) and (B.Top < A.Top + A.Height)), 'Overlapping controls');
      end;
    end;
    AssertTrue(CancelButton.Cancel, 'Escape must cancel cleanup');
    Metrics := Metrics + Format('%s font=%d targets=%d dialog=%dx%d', [ActiveLanguage, Form.Font.Size, FixtureTargets, Form.ClientWidth, Form.ClientHeight]) + #13#10;
    if Installers <> nil then Installers.Checked := (FixtureSelected and 1) <> 0;
    if Versions <> nil then Versions.Checked := (FixtureSelected and 2) <> 0;
    if Backups <> nil then Backups.Checked := (FixtureSelected and 4) <> 0;
    if FixtureCancel then Result := mrCancel else Result := mrOk;
  finally
    Bitmap.Free;
  end;
end;

${dialog}
${functionSource("ExistingInstallDir")}
${functionSource("ShouldSkipPage")}

procedure InitializeWizard;
var I, WelcomeHeight: Integer;
begin
  try
    WelcomeHeight := WizardForm.WelcomeLabel2.Height;
    WizardForm.WelcomeLabel2.Caption := FmtMessage(CustomMessage('WelcomeDescription'), ['1.0.0.0', 'amd64']);
    WizardForm.WelcomeLabel2.AdjustHeight;
    AssertTrue(WizardForm.WelcomeLabel2.Height <= WelcomeHeight, 'Welcome description clipped');
    InitializePreparingProgress;
    SetPreparingStatus(CustomMessage('CheckingRuntimes'));
    AssertTrue(PreparingProgress.Top + PreparingProgress.Height <= WizardForm.PreparingPage.ClientHeight, 'Preparing progress outside page');
    SetPreparingStatus(FmtMessage(CustomMessage('RuntimeInstallerBusy'), ['60']));
    AssertTrue(PreparingProgress.Top >= WizardForm.PreparingLabel.Top + WizardForm.PreparingLabel.Height, 'Progress overlaps status');
    AssertTrue(PreparingProgress.Top + PreparingProgress.Height <= WizardForm.PreparingPage.ClientHeight, 'Waiting progress outside page');
    StopPreparingProgress;
    AssertTrue(not PreparingProgress.Visible, 'Progress must stop on exit');
    AssertTrue(WizardForm.PreparingLabel.Height = PreparingLabelHeight, 'Restore native error layout');
    CurrentInnoInstallDir := '';
    LegacyNsisInstallDir := '';
    AssertTrue(not ShouldSkipPage(wpWelcome), 'Fresh install needs welcome');
    AssertTrue(not ShouldSkipPage(wpSelectDir), 'Fresh install needs directory choice');
    CurrentInnoInstallDir := 'C:\Fixture';
    AssertTrue(ShouldSkipPage(wpWelcome) and ShouldSkipPage(wpSelectDir), 'Inno upgrade skips welcome and directory');
    CurrentInnoInstallDir := '';
    LegacyNsisInstallDir := 'C:\Legacy';
    AssertTrue(ShouldSkipPage(wpWelcome) and ShouldSkipPage(wpSelectDir), 'NSIS migration skips welcome and directory');
    AssertTrue(not ShouldSkipPage(wpInfoBefore), 'Upgrade keeps license information');
    for I := 1 to 7 do
    begin
      FixtureTargets := I;
      FixtureSelected := 0;
      FixtureCancel := False;
      AssertTrue(ShowUninstallOptions('C:\Fixture\实例库\Library'), 'Continue without cleanup');
      AssertTrue(not (RemoveInstallers or RemoveVersions or RemoveBackups), 'Default keeps all data');
      FixtureSelected := 7;
      AssertTrue(ShowUninstallOptions('C:\Fixture'), 'Continue with selected cleanup');
      AssertTrue(RemoveInstallers = ((I and 1) <> 0), 'Only existing installers may be selected');
      AssertTrue(RemoveVersions = ((I and 2) <> 0), 'Only existing versions may be selected');
      AssertTrue(RemoveBackups = ((I and 4) <> 0), 'Only existing backups may be selected');
      FixtureCancel := True;
      AssertTrue(not ShowUninstallOptions('C:\Fixture'), 'Cancel aborts uninstall');
      AssertTrue(not (RemoveInstallers or RemoveVersions or RemoveBackups), 'Cancel keeps all data');
    end;
    SaveStringToFile(ExpandConstant('{src}\result.txt'), 'PASS', False);
    SaveStringToFile(ExpandConstant('{src}\metrics.txt'), Metrics, False);
  except
    SaveStringToFile(ExpandConstant('{src}\result.txt'), 'FAIL: ' + GetExceptionMessage, False);
  end;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  // Silent Setup exits before installation when this event returns False.
  Result := False;
end;
`;
await writeFile(join(directory, "tests.iss"), harness, "utf8");
for (const size of [9, 12, 18]) {
  execFileSync(compiler, ["/Q", `/DTestFontSize=${size}`, join(directory, "tests.iss")], { windowsHide: true, stdio: "inherit" });
  for (const language of languages) {
    await writeFile(join(directory, "result.txt"), "NOT RUN", "utf8");
    const result = spawnSync(join(directory, "tests.exe"), [`/LANG=${language}`, "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"], { windowsHide: true, timeout: 15_000 });
    if (result.error) throw result.error;
    assert.equal(await readFile(join(directory, "result.txt"), "utf8"), "PASS", `${language}, ${size}pt`);
    await writeFile(join(directory, `${language}-${size}pt.txt`), await readFile(join(directory, "metrics.txt")));
  }
}
console.log(`Native wizard layout and cleanup tests passed: ${languages.length} languages, 3 font sizes, 7 folder combinations (${directory})`);
