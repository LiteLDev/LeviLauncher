// Included after the main installer's global declarations.
const
  WebView2Key = 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  VCRuntimeKey = 'Software\Microsoft\VisualStudio\14.0\VC\Runtimes\{#VCRuntimeArch}';
  // ERROR_INSTALL_ALREADY_RUNNING. Another Windows Installer transaction owns the
  // machine; the condition clears on its own, so this code is waited out instead
  // of ending the wizard on the Preparing page.
  ErrorInstallAlreadyRunning = 1618;
  ErrorSuccessRebootRequired = 3010;
  ErrorSuccessRebootInitiated = 1641;
  BusyInstallerPollMs = 500;
  BusyInstallerRetryDelayMs = 3000;
  BusyInstallerRetryLimit = 20;

var
  RuntimeRestartRequired: Boolean;
  PreparingProgress: TNewProgressBar;
  PreparingLabelHeight: Integer;

procedure InitializePreparingProgress;
begin
  PreparingLabelHeight := WizardForm.PreparingLabel.Height;
  PreparingProgress := TNewProgressBar.Create(WizardForm);
  PreparingProgress.Parent := WizardForm.PreparingPage;
  PreparingProgress.Left := WizardForm.PreparingLabel.Left;
  PreparingProgress.Width := WizardForm.PreparingLabel.Width;
  PreparingProgress.Height := ScaleY(8);
  PreparingProgress.Style := npbstMarquee;
  PreparingProgress.Visible := False;
end;

procedure SetPreparingStage(const StageMessage: string);
begin
  if WizardSilent then
    exit;
  WizardForm.PageNameLabel.Caption := StageMessage;
  WizardForm.PageNameLabel.Update;
end;

procedure SetPreparingStatus(const StatusMessage: string);
begin
  WizardForm.PreparingLabel.Caption := StatusMessage;
  WizardForm.PreparingLabel.Update;
  if PreparingProgress <> nil then
  begin
    WizardForm.PreparingLabel.AdjustHeight;
    PreparingProgress.Top := WizardForm.PreparingLabel.Top + WizardForm.PreparingLabel.Height + ScaleY(16);
    PreparingProgress.Visible := not WizardSilent;
    PreparingProgress.Update;
  end;
end;

procedure StopPreparingProgress;
begin
  if PreparingProgress <> nil then
  begin
    PreparingProgress.Visible := False;
    // Inno owns the failure/reboot layout after PrepareToInstall returns.
    WizardForm.PreparingLabel.Height := PreparingLabelHeight;
  end;
end;

function HasWebView2Runtime(RootKey: Integer): Boolean;
var
  Version: string;
  PackedVersion: Int64;
begin
  Result := False;
  if RegQueryStringValue(RootKey, WebView2Key, 'pv', Version) then
    if StrToVersion(Trim(Version), PackedVersion) then
      Result := PackedVersion > 0;
end;

function NeedsWebView2Runtime: Boolean;
begin
  // Edge Update registers machine installs in the 32-bit registry view.
  // This installer is elevated and installs for all users: an administrator's
  // HKCU runtime does not guarantee availability for the original/other users.
  Result := not HasWebView2Runtime(HKLM32);
end;

// Mirrors the launcher's own runtime gate in internal/vcruntime/vcruntime.go: any
// registered v14 redistributable satisfies it. Requiring the bundled package's
// exact version instead would reinstall the runtime on nearly every machine and
// expose every install to transient Windows Installer failures.
function HasVCRuntime(RootKey: Integer): Boolean;
var
  Installed, Major: Cardinal;
  Version: string;
begin
  Result := False;
  if not RegQueryDWordValue(RootKey, VCRuntimeKey, 'Installed', Installed) then
    exit;
  if Installed <> 1 then
    exit;
  if RegQueryStringValue(RootKey, VCRuntimeKey, 'Version', Version) and (Trim(Version) <> '') then
  begin
    Result := True;
    exit;
  end;
  Result := RegQueryDWordValue(RootKey, VCRuntimeKey, 'Major', Major) and (Major >= 14);
end;

function NeedsVCRuntime: Boolean;
begin
  Result := not (HasVCRuntime(HKLM32) or HasVCRuntime(HKLM64));
end;

procedure WaitForBusyInstaller(RemainingAttempts: Integer);
var
  Elapsed, RemainingSeconds: Integer;
begin
  Elapsed := 0;
  while Elapsed < BusyInstallerRetryDelayMs do
  begin
    RemainingSeconds := ((RemainingAttempts * BusyInstallerRetryDelayMs) - Elapsed) div 1000;
    SetPreparingStatus(FmtMessage(CustomMessage('RuntimeInstallerBusy'), [IntToStr(RemainingSeconds)]));
    Sleep(BusyInstallerPollMs);
    Elapsed := Elapsed + BusyInstallerPollMs;
  end;
end;

function RunRuntimeInstaller(const FileName, Parameters, StatusMessage: string; var ResultCode: Integer): Boolean;
var
  Attempt: Integer;
begin
  Result := False;
  for Attempt := 0 to BusyInstallerRetryLimit do
  begin
    SetPreparingStatus(StatusMessage);
    Result := Exec(ExpandConstant('{tmp}\') + FileName, Parameters, '',
      SW_HIDE, ewWaitUntilTerminated, ResultCode);
    if (not Result) or (ResultCode <> ErrorInstallAlreadyRunning) then
      exit;
    Log(Format('%s is waiting for another installation to finish (attempt %d)', [FileName, Attempt + 1]));
    if Attempt < BusyInstallerRetryLimit then
      WaitForBusyInstaller(BusyInstallerRetryLimit - Attempt);
  end;
end;

function InstallRuntime(const FileName, Parameters, DisplayName, StatusMessage: string): string;
var
  ResultCode: Integer;
begin
  Result := '';
  SetPreparingStage(CustomMessage('PreparingComponents'));
  SetPreparingStatus(StatusMessage);
  ExtractTemporaryFile(FileName);
  if not RunRuntimeInstaller(FileName, Parameters, StatusMessage, ResultCode) then
  begin
    Log(Format('%s installer could not be started: %d', [DisplayName, ResultCode]));
    Result := FmtMessage(CustomMessage('RuntimeInstallFailed'), [DisplayName, IntToStr(ResultCode)]);
    exit;
  end;
  Log(Format('%s installer exit code: %d', [DisplayName, ResultCode]));
  if (ResultCode = ErrorSuccessRebootRequired) or (ResultCode = ErrorSuccessRebootInitiated) then
    RuntimeRestartRequired := True
  else if ResultCode <> 0 then
    Result := FmtMessage(CustomMessage('RuntimeInstallFailed'), [DisplayName, IntToStr(ResultCode)]);
end;

function InstallPrerequisites: string;
begin
  Result := '';
  try
    SetPreparingStage(CustomMessage('PreparingEnvironment'));
    SetPreparingStatus(CustomMessage('CheckingRuntimes'));
    if NeedsVCRuntime() then
    begin
      Result := InstallRuntime('{#VCRuntimeFile}', '/install /quiet /norestart',
        'Microsoft Visual C++ Runtime', CustomMessage('InstallVCRuntime'));
      if Result <> '' then
        exit;
      if NeedsVCRuntime() then
      begin
        Result := FmtMessage(CustomMessage('RuntimeNotDetected'), ['Microsoft Visual C++ Runtime']);
        exit;
      end;
    end;

    if NeedsWebView2Runtime() then
    begin
      Result := InstallRuntime('MicrosoftEdgeWebview2Setup.exe', '/silent /install',
        'WebView2 Runtime', CustomMessage('InstallWebView2'));
      if Result <> '' then
        exit;
      if NeedsWebView2Runtime() then
        Result := FmtMessage(CustomMessage('RuntimeNotDetected'), ['WebView2 Runtime']);
    end;
    if Result = '' then
    begin
      SetPreparingStage(CustomMessage('PreparingLauncher'));
      SetPreparingStatus(CustomMessage('RuntimesReady'));
    end;
  except
    Result := GetExceptionMessage;
    Log(Result);
  end;
end;

function PrepareToInstall(var NeedsRestart: Boolean): string;
begin
  Result := InstallPrerequisites();
  StopPreparingProgress;
  // A non-empty result ends the wizard on the Preparing page, so the user gets to
  // clear the blocker and retry here instead of restarting setup from scratch.
  while (Result <> '') and not WizardSilent do
  begin
    if MsgBox(Result + #13#10#13#10 + CustomMessage('RuntimeInstallRetry'), mbError, MB_RETRYCANCEL) <> IDRETRY then
      break;
    Result := InstallPrerequisites();
    StopPreparingProgress;
  end;
  // On success NeedRestart handles the final prompt. On failure the Preparing
  // page must also report a pending prerequisite reboot before allowing retry.
  NeedsRestart := RuntimeRestartRequired and (Result <> '');
end;

function NeedRestart: Boolean;
begin
  Result := RuntimeRestartRequired;
end;
