// Included after the main installer's global declarations.
const
  WebView2Key = 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  VCRuntimeKey = 'Software\Microsoft\VisualStudio\14.0\VC\Runtimes\{#VCRuntimeArch}';

var
  RuntimeRestartRequired: Boolean;

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

function HasVCRuntime(RootKey: Integer): Boolean;
var
  Installed: Cardinal;
  Version: string;
  PackedVersion, RequiredVersion: Int64;
begin
  Result := False;
  if not RegQueryDWordValue(RootKey, VCRuntimeKey, 'Installed', Installed) then
    exit;
  if Installed <> 1 then
    exit;
  if not RegQueryStringValue(RootKey, VCRuntimeKey, 'Version', Version) then
    exit;
  Version := Trim(Version);
  if (Length(Version) > 0) and ((Version[1] = 'v') or (Version[1] = 'V')) then
    Delete(Version, 1, 1);
  if StrToVersion(Version, PackedVersion) and
     StrToVersion('{#VCRuntimeVersion}', RequiredVersion) then
    Result := ComparePackedVersion(PackedVersion, RequiredVersion) >= 0;
end;

function NeedsVCRuntime: Boolean;
begin
  Result := not (HasVCRuntime(HKLM32) or HasVCRuntime(HKLM64));
end;

function InstallRuntime(const FileName, Parameters, DisplayName, StatusMessage: string): string;
var
  ResultCode: Integer;
begin
  Result := '';
  WizardForm.PreparingLabel.Caption := StatusMessage;
  ExtractTemporaryFile(FileName);
  if not Exec(ExpandConstant('{tmp}\') + FileName, Parameters, '',
      SW_HIDE, ewWaitUntilTerminated, ResultCode) then
    Result := FmtMessage(CustomMessage('RuntimeInstallFailed'), [DisplayName, IntToStr(ResultCode)])
  else if (ResultCode = 3010) or (ResultCode = 1641) then
    RuntimeRestartRequired := True
  else if ResultCode <> 0 then
    Result := FmtMessage(CustomMessage('RuntimeInstallFailed'), [DisplayName, IntToStr(ResultCode)]);
  Log(Format('%s installer exit code: %d', [DisplayName, ResultCode]));
end;

function InstallPrerequisites: string;
begin
  Result := '';
  try
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
  except
    Result := GetExceptionMessage;
    Log(Result);
  end;
end;

function PrepareToInstall(var NeedsRestart: Boolean): string;
begin
  Result := InstallPrerequisites();
  // On success NeedRestart handles the final prompt. On failure the Preparing
  // page must also report a pending prerequisite reboot before allowing retry.
  NeedsRestart := RuntimeRestartRequired and (Result <> '');
end;

function NeedRestart: Boolean;
begin
  Result := RuntimeRestartRequired;
end;
