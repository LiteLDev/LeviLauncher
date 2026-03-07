#ifndef AppName
  #define AppName "LeviLauncher"
#endif

#ifndef AppPublisher
  #define AppPublisher "LeviMC"
#endif

#ifndef AppArch
  #define AppArch "amd64"
#endif

#ifndef AppBinaryPath
  #error "AppBinaryPath is required. Example: /DAppBinaryPath=C:\path\LeviLauncher.exe"
#endif

#define AppExeName AppName + ".exe"
#define AppVersion GetVersionNumbersString(AppBinaryPath)

#if AppVersion == ""
  #define AppVersion "0.0.0.0"
#endif

#if AppArch == "amd64"
  #define AllowedArchitectures "x64compatible"
#elif AppArch == "arm64"
  #define AllowedArchitectures "arm64"
#else
  #error "Unsupported AppArch. Use amd64 or arm64."
#endif

[Setup]
AppId=org.levimc.launcher
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppCopyright=(c) 2024-2026, LeviMC
DefaultDirName={code:GetDefaultInstallDir|{autopf64}\{#AppPublisher}\{#AppName}}
DefaultGroupName={#AppName}
PrivilegesRequired=admin
OutputDir=..\..\..\bin
OutputBaseFilename={#AppName}-{#AppArch}-installer
SetupIconFile=..\icon.ico
UninstallDisplayIcon={app}\{#AppExeName}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
DisableDirPage=auto
UsePreviousAppDir=no
UsePreviousGroup=yes
ArchitecturesAllowed={#AllowedArchitectures}
ArchitecturesInstallIn64BitMode={#AllowedArchitectures}
VersionInfoVersion={#AppVersion}
VersionInfoCompany={#AppPublisher}
VersionInfoDescription={#AppName} Installer
VersionInfoCopyright=(c) 2024-2026, LeviMC
VersionInfoProductName={#AppName}
VersionInfoProductVersion={#AppVersion}
ShowLanguageDialog=auto

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "simpchinese"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"

[CustomMessages]
english.InstallWebView2=Installing WebView2 Runtime...
simpchinese.InstallWebView2=正在安装 WebView2 运行时...
english.MsgCloseBeforeUninstall={#AppName} is currently running. Please close it before uninstalling.
simpchinese.MsgCloseBeforeUninstall={#AppName} 正在运行。请先关闭后再卸载。
english.UninstallOptionsTitle=Optional Data Cleanup
simpchinese.UninstallOptionsTitle=可选数据清理
english.UninstallOptionsDesc=BaseRoot is your instance library and may contain instances, saves, mods, and resource packs.
simpchinese.UninstallOptionsDesc=BaseRoot 是你的实例库，可能包含实例、存档、模组和资源包。
english.UninstallDefaultSafe=Default behavior keeps all BaseRoot data intact.
simpchinese.UninstallDefaultSafe=默认行为会保留 BaseRoot 下的全部数据。
english.UninstallDetectedBaseRoot=Detected BaseRoot:
simpchinese.UninstallDetectedBaseRoot=检测到 BaseRoot：
english.UninstallDangerNote=Caution: deleting these folders may remove game files or user data depending on your layout.
simpchinese.UninstallDangerNote=注意：删除以下目录可能导致游戏文件或用户数据丢失（取决于你的目录布局）。
english.UninstallSelectWhatToDelete=Select folders to delete:
simpchinese.UninstallSelectWhatToDelete=请选择要删除的目录：
english.UninstallDeleteInstallers=Delete "installers" (installer package directory)
simpchinese.UninstallDeleteInstallers=删除 "installers"（安装包目录）
english.UninstallDeleteVersions=[Dangerous] Delete "versions" (game instance directory)
simpchinese.UninstallDeleteVersions=[高风险] 删除 "versions"（游戏实例目录）
english.UninstallDeleteBackups=Delete "backups" (backup directory)
simpchinese.UninstallDeleteBackups=删除 "backups"（备份目录）
english.UninstallFolderNotFound=(not found)
simpchinese.UninstallFolderNotFound=（未找到）
english.UninstallNoOptionalData=No optional BaseRoot cleanup targets were found. Keeping all instance data.
simpchinese.UninstallNoOptionalData=未发现可选的 BaseRoot 清理目标，实例数据将全部保留。
english.UpgradeModeDesc=Upgrade mode detected. Existing installation path:%n%1%nThis setup will upgrade in place and keep your current installation directory.
simpchinese.UpgradeModeDesc=检测到升级模式。现有安装路径：%n%1%n本次将执行原地升级，并保留当前安装目录。
english.NsisMigrationModeDesc=Upgrade mode detected. Existing installation path:%n%1%nThis setup will upgrade in place and keep your current installation directory.
simpchinese.NsisMigrationModeDesc=检测到升级模式。现有安装路径：%n%1%n本次将执行原地升级，并保留当前安装目录。

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: checkedonce

[Files]
Source: "{#AppBinaryPath}"; DestDir: "{app}"; DestName: "{#AppExeName}"; Flags: ignoreversion
Source: "MicrosoftEdgeWebview2Setup.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall; Check: NeedsWebView2Runtime

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Run]
Filename: "{tmp}\MicrosoftEdgeWebview2Setup.exe"; Parameters: "/silent /install"; Flags: runhidden waituntilterminated; StatusMsg: "{cm:InstallWebView2}"; Check: NeedsWebView2Runtime

[Code]
const
  WebView2ClientId = '{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  WebView2HklmPath = 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\' + WebView2ClientId;
  WebView2HkcuPath = 'Software\Microsoft\EdgeUpdate\Clients\' + WebView2ClientId;
  CurrentInnoUninstallKey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\org.levimc.launcher_is1';
  LegacyNsisUninstallKey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{#AppPublisher}{#AppName}';
  LegacyNsisUninstallKeyAlt = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{#AppName}';

var
  UninstallBaseRoot: string;
  RemoveInstallers: Boolean;
  RemoveVersions: Boolean;
  RemoveBackups: Boolean;
  CurrentInnoInstallDir: string;
  LegacyNsisInstallDir: string;

function NeedsWebView2Runtime: Boolean;
var
  Version: string;
begin
  if RegQueryStringValue(HKLM64, WebView2HklmPath, 'pv', Version) and (Trim(Version) <> '') then
  begin
    Result := False;
    exit;
  end;

  if RegQueryStringValue(HKCU, WebView2HkcuPath, 'pv', Version) and (Trim(Version) <> '') then
  begin
    Result := False;
    exit;
  end;

  Result := True;
end;

function ExtractPathFromCommand(const CommandValue: string): string;
var
  RawValue: string;
  ClosingQuotePos: Integer;
  FirstSpacePos: Integer;
begin
  Result := '';
  RawValue := Trim(CommandValue);
  if RawValue = '' then
    exit;

  if RawValue[1] = '"' then
  begin
    Delete(RawValue, 1, 1);
    ClosingQuotePos := Pos('"', RawValue);
    if ClosingQuotePos > 0 then
      RawValue := Copy(RawValue, 1, ClosingQuotePos - 1);
  end
  else
  begin
    FirstSpacePos := Pos(' ', RawValue);
    if FirstSpacePos > 0 then
      RawValue := Copy(RawValue, 1, FirstSpacePos - 1);
  end;

  Result := ExtractFileDir(RawValue);
end;

function TryResolveInstallDirFromUninstallKey(RootKey: Integer; const UninstallKey: string; var InstallDir: string): Boolean;
var
  Candidate: string;
begin
  Result := False;
  InstallDir := '';

  if RegQueryStringValue(RootKey, UninstallKey, 'InstallLocation', Candidate) and (Trim(Candidate) <> '') then
  begin
    InstallDir := Trim(Candidate);
    if DirExists(InstallDir) then
    begin
      Result := True;
      exit;
    end;
  end;

  if RegQueryStringValue(RootKey, UninstallKey, 'Inno Setup: App Path', Candidate) and (Trim(Candidate) <> '') then
  begin
    InstallDir := Trim(Candidate);
    if DirExists(InstallDir) then
    begin
      Result := True;
      exit;
    end;
  end;

  if RegQueryStringValue(RootKey, UninstallKey, 'DisplayIcon', Candidate) and (Trim(Candidate) <> '') then
  begin
    InstallDir := ExtractPathFromCommand(Candidate);
    if (InstallDir <> '') and DirExists(InstallDir) then
    begin
      Result := True;
      exit;
    end;
  end;

  if RegQueryStringValue(RootKey, UninstallKey, 'UninstallString', Candidate) and (Trim(Candidate) <> '') then
  begin
    InstallDir := ExtractPathFromCommand(Candidate);
    if (InstallDir <> '') and DirExists(InstallDir) then
    begin
      Result := True;
      exit;
    end;
  end;
end;

function ResolveLegacyNsisInstallDir: string;
var
  LegacyDir: string;
begin
  Result := '';
  if TryResolveInstallDirFromUninstallKey(HKLM64, LegacyNsisUninstallKey, LegacyDir) then
  begin
    Result := LegacyDir;
    exit;
  end;

  if TryResolveInstallDirFromUninstallKey(HKCU, LegacyNsisUninstallKey, LegacyDir) then
  begin
    Result := LegacyDir;
    exit;
  end;

  if TryResolveInstallDirFromUninstallKey(HKLM64, LegacyNsisUninstallKeyAlt, LegacyDir) then
  begin
    Result := LegacyDir;
    exit;
  end;

  if TryResolveInstallDirFromUninstallKey(HKCU, LegacyNsisUninstallKeyAlt, LegacyDir) then
  begin
    Result := LegacyDir;
    exit;
  end;
end;

function ResolveCurrentInnoInstallDir: string;
var
  CurrentDir: string;
begin
  Result := '';
  if TryResolveInstallDirFromUninstallKey(HKLM64, CurrentInnoUninstallKey, CurrentDir) then
  begin
    Result := CurrentDir;
    exit;
  end;

  if TryResolveInstallDirFromUninstallKey(HKCU, CurrentInnoUninstallKey, CurrentDir) then
  begin
    Result := CurrentDir;
    exit;
  end;
end;

function GetDefaultInstallDir(DefaultDir: string): string;
var
  CurrentDir: string;
  LegacyDir: string;
begin
  CurrentDir := ResolveCurrentInnoInstallDir();
  if CurrentDir <> '' then
  begin
    Result := CurrentDir;
    exit;
  end;

  LegacyDir := ResolveLegacyNsisInstallDir();
  if LegacyDir <> '' then
    Result := LegacyDir
  else
    Result := DefaultDir;
end;

function NormalizeDirForCompare(const Dir: string): string;
begin
  Result := UpperCase(Trim(Dir));
  while (Length(Result) > 0) and ((Result[Length(Result)] = '\') or (Result[Length(Result)] = '/')) do
  begin
    if (Length(Result) = 3) and (Result[2] = ':') then
      break;
    Delete(Result, Length(Result), 1);
  end;
end;

function ShouldCleanupLegacyNsisUninstaller(): Boolean;
var
  TargetDir: string;
begin
  Result := False;
  if LegacyNsisInstallDir = '' then
    LegacyNsisInstallDir := ResolveLegacyNsisInstallDir();
  if LegacyNsisInstallDir = '' then
    exit;

  TargetDir := WizardDirValue();
  if TargetDir = '' then
    exit;

  Result := NormalizeDirForCompare(LegacyNsisInstallDir) = NormalizeDirForCompare(TargetDir);
end;

function IsAppRunning: Boolean;
var
  ResultCode: Integer;
begin
  Result :=
    Exec(
      ExpandConstant('{cmd}'),
      '/C tasklist /FI "IMAGENAME eq {#AppExeName}" | find /I "{#AppExeName}" >NUL',
      '',
      SW_HIDE,
      ewWaitUntilTerminated,
      ResultCode
    ) and (ResultCode = 0);
end;

function IsSilentUninstall: Boolean;
var
  CmdTailUpper: string;
begin
  CmdTailUpper := UpperCase(GetCmdTail());
  Result := (Pos('/SILENT', CmdTailUpper) > 0) or (Pos('/VERYSILENT', CmdTailUpper) > 0);
end;

function ExtractJsonStringValue(const Json: string; const Key: string): string;
var
  KeyPattern: string;
  Cursor: Integer;
  Ch: Char;
  Escaped: Boolean;
begin
  Result := '';
  KeyPattern := '"' + Key + '"';
  Cursor := Pos(KeyPattern, Json);
  if Cursor = 0 then
    exit;

  Cursor := Cursor + Length(KeyPattern);
  while (Cursor <= Length(Json)) and (Json[Cursor] <> ':') do
    Cursor := Cursor + 1;
  if Cursor > Length(Json) then
    exit;

  Cursor := Cursor + 1;
  while (Cursor <= Length(Json)) and (Json[Cursor] <= ' ') do
    Cursor := Cursor + 1;
  if (Cursor > Length(Json)) or (Json[Cursor] <> '"') then
    exit;

  Cursor := Cursor + 1;
  Escaped := False;
  while Cursor <= Length(Json) do
  begin
    Ch := Json[Cursor];
    if Escaped then
    begin
      if Ch = 'n' then
        Result := Result + #10
      else if Ch = 'r' then
        Result := Result + #13
      else if Ch = 't' then
        Result := Result + #9
      else
        Result := Result + Ch;
      Escaped := False;
    end
    else if Ch = '\' then
    begin
      Escaped := True;
    end
    else if Ch = '"' then
    begin
      exit;
    end
    else
    begin
      Result := Result + Ch;
    end;
    Cursor := Cursor + 1;
  end;
end;

function ResolveBaseRootFromConfig: string;
var
  LocalPath: string;
  ConfigPath: string;
  ConfigJsonRaw: AnsiString;
  ConfigJson: string;
begin
  Result := '';
  LocalPath := ExpandConstant('{userappdata}\{#AppExeName}');
  ConfigPath := LocalPath + '\config.json';

  if LoadStringFromFile(ConfigPath, ConfigJsonRaw) then
  begin
    ConfigJson := ConfigJsonRaw;
    Result := Trim(ExtractJsonStringValue(ConfigJson, 'base_root'));
  end;
end;

function HasOptionalBaseRootData(const BaseRoot: string): Boolean;
begin
  Result :=
    (BaseRoot <> '') and
    (
      DirExists(BaseRoot + '\installers') or
      DirExists(BaseRoot + '\versions') or
      DirExists(BaseRoot + '\backups')
    );
end;

function ShowUninstallOptions(const BaseRoot: string): Boolean;
var
  OptionsForm: TSetupForm;
  TitleLabel: TNewStaticText;
  DescLabel: TNewStaticText;
  SafeLabel: TNewStaticText;
  WarningLabel: TNewStaticText;
  BaseRootHeaderLabel: TNewStaticText;
  BaseRootPathEdit: TNewEdit;
  ChoiceHeaderLabel: TNewStaticText;
  RemoveInstallersCheck: TNewCheckBox;
  RemoveVersionsCheck: TNewCheckBox;
  RemoveBackupsCheck: TNewCheckBox;
  OkButton: TNewButton;
  CancelButton: TNewButton;
  HasInstallers: Boolean;
  HasVersions: Boolean;
  HasBackups: Boolean;
  CurTop: Integer;
  ButtonsTop: Integer;
begin
  Result := True;
  RemoveInstallers := False;
  RemoveVersions := False;
  RemoveBackups := False;
  HasInstallers := DirExists(BaseRoot + '\installers');
  HasVersions := DirExists(BaseRoot + '\versions');
  HasBackups := DirExists(BaseRoot + '\backups');

  OptionsForm := CreateCustomForm(ScaleX(500), ScaleY(320), False, False);
  try
    OptionsForm.Caption := ExpandConstant('{cm:UninstallOptionsTitle}');

    TitleLabel := TNewStaticText.Create(OptionsForm);
    TitleLabel.Parent := OptionsForm;
    TitleLabel.Left := ScaleX(16);
    TitleLabel.Top := ScaleY(8);
    TitleLabel.Caption := ExpandConstant('{cm:UninstallOptionsTitle}');
    TitleLabel.Font.Style := [fsBold];

    DescLabel := TNewStaticText.Create(OptionsForm);
    DescLabel.Parent := OptionsForm;
    DescLabel.Left := ScaleX(16);
    DescLabel.Top := TitleLabel.Top + TitleLabel.Height + ScaleY(5);
    DescLabel.Caption := ExpandConstant('{cm:UninstallOptionsDesc}');

    SafeLabel := TNewStaticText.Create(OptionsForm);
    SafeLabel.Parent := OptionsForm;
    SafeLabel.Left := ScaleX(16);
    SafeLabel.Top := DescLabel.Top + DescLabel.Height + ScaleY(3);
    SafeLabel.Caption := ExpandConstant('{cm:UninstallDefaultSafe}');

    WarningLabel := TNewStaticText.Create(OptionsForm);
    WarningLabel.Parent := OptionsForm;
    WarningLabel.Left := ScaleX(16);
    WarningLabel.Top := SafeLabel.Top + SafeLabel.Height + ScaleY(3);
    WarningLabel.Caption := ExpandConstant('{cm:UninstallDangerNote}');
    WarningLabel.Font.Style := [fsBold];

    BaseRootHeaderLabel := TNewStaticText.Create(OptionsForm);
    BaseRootHeaderLabel.Parent := OptionsForm;
    BaseRootHeaderLabel.Left := ScaleX(16);
    BaseRootHeaderLabel.Top := WarningLabel.Top + WarningLabel.Height + ScaleY(8);
    BaseRootHeaderLabel.Caption := ExpandConstant('{cm:UninstallDetectedBaseRoot}');

    BaseRootPathEdit := TNewEdit.Create(OptionsForm);
    BaseRootPathEdit.Parent := OptionsForm;
    BaseRootPathEdit.Left := ScaleX(16);
    BaseRootPathEdit.Top := BaseRootHeaderLabel.Top + BaseRootHeaderLabel.Height + ScaleY(3);
    BaseRootPathEdit.Width := OptionsForm.ClientWidth - ScaleX(32);
    BaseRootPathEdit.Height := ScaleY(22);
    BaseRootPathEdit.ReadOnly := True;
    BaseRootPathEdit.Text := BaseRoot;

    ChoiceHeaderLabel := TNewStaticText.Create(OptionsForm);
    ChoiceHeaderLabel.Parent := OptionsForm;
    ChoiceHeaderLabel.Left := ScaleX(16);
    ChoiceHeaderLabel.Top := BaseRootPathEdit.Top + BaseRootPathEdit.Height + ScaleY(8);
    ChoiceHeaderLabel.Caption := ExpandConstant('{cm:UninstallSelectWhatToDelete}');
    ChoiceHeaderLabel.Font.Style := [fsBold];
    ChoiceHeaderLabel.Height := ScaleY(22);
    ChoiceHeaderLabel.AutoSize := False;

    CurTop := ChoiceHeaderLabel.Top + ChoiceHeaderLabel.Height + ScaleY(4);

    RemoveInstallersCheck := TNewCheckBox.Create(OptionsForm);
    RemoveInstallersCheck.Parent := OptionsForm;
    RemoveInstallersCheck.Left := ScaleX(16);
    RemoveInstallersCheck.Top := CurTop;
    RemoveInstallersCheck.Width := OptionsForm.ClientWidth - ScaleX(32);
    RemoveInstallersCheck.Height := ScaleY(24);
    RemoveInstallersCheck.Caption := ExpandConstant('{cm:UninstallDeleteInstallers}');
    RemoveInstallersCheck.Checked := False;
    RemoveInstallersCheck.Enabled := HasInstallers;
    if not HasInstallers then
      RemoveInstallersCheck.Caption := RemoveInstallersCheck.Caption + ' ' + ExpandConstant('{cm:UninstallFolderNotFound}');
    CurTop := RemoveInstallersCheck.Top + RemoveInstallersCheck.Height + ScaleY(2);

    RemoveVersionsCheck := TNewCheckBox.Create(OptionsForm);
    RemoveVersionsCheck.Parent := OptionsForm;
    RemoveVersionsCheck.Left := ScaleX(16);
    RemoveVersionsCheck.Top := CurTop;
    RemoveVersionsCheck.Width := OptionsForm.ClientWidth - ScaleX(32);
    RemoveVersionsCheck.Height := ScaleY(24);
    RemoveVersionsCheck.Caption := ExpandConstant('{cm:UninstallDeleteVersions}');
    RemoveVersionsCheck.Checked := False;
    RemoveVersionsCheck.Enabled := HasVersions;
    if not HasVersions then
      RemoveVersionsCheck.Caption := RemoveVersionsCheck.Caption + ' ' + ExpandConstant('{cm:UninstallFolderNotFound}');
    CurTop := RemoveVersionsCheck.Top + RemoveVersionsCheck.Height + ScaleY(2);

    RemoveBackupsCheck := TNewCheckBox.Create(OptionsForm);
    RemoveBackupsCheck.Parent := OptionsForm;
    RemoveBackupsCheck.Left := ScaleX(16);
    RemoveBackupsCheck.Top := CurTop;
    RemoveBackupsCheck.Width := OptionsForm.ClientWidth - ScaleX(32);
    RemoveBackupsCheck.Height := ScaleY(24);
    RemoveBackupsCheck.Caption := ExpandConstant('{cm:UninstallDeleteBackups}');
    RemoveBackupsCheck.Checked := False;
    RemoveBackupsCheck.Enabled := HasBackups;
    if not HasBackups then
      RemoveBackupsCheck.Caption := RemoveBackupsCheck.Caption + ' ' + ExpandConstant('{cm:UninstallFolderNotFound}');

    ButtonsTop := OptionsForm.ClientHeight - ScaleY(30);
    if ButtonsTop < (RemoveBackupsCheck.Top + RemoveBackupsCheck.Height + ScaleY(10)) then
    begin
      ButtonsTop := RemoveBackupsCheck.Top + RemoveBackupsCheck.Height + ScaleY(10);
      OptionsForm.ClientHeight := ButtonsTop + ScaleY(30);
    end;

    OkButton := TNewButton.Create(OptionsForm);
    OkButton.Parent := OptionsForm;
    OkButton.Caption := SetupMessage(msgButtonOK);
    OkButton.ModalResult := mrOk;
    OkButton.Default := True;
    OkButton.Width := ScaleX(88);
    OkButton.Height := ScaleY(24);
    OkButton.Left := OptionsForm.ClientWidth - ScaleX(194);
    OkButton.Top := ButtonsTop;

    CancelButton := TNewButton.Create(OptionsForm);
    CancelButton.Parent := OptionsForm;
    CancelButton.Caption := SetupMessage(msgButtonCancel);
    CancelButton.ModalResult := mrCancel;
    CancelButton.Cancel := True;
    CancelButton.Width := ScaleX(88);
    CancelButton.Height := ScaleY(24);
    CancelButton.Left := OptionsForm.ClientWidth - ScaleX(98);
    CancelButton.Top := ButtonsTop;

    if OptionsForm.ShowModal <> mrOk then
    begin
      Result := False;
      exit;
    end;

    RemoveInstallers := RemoveInstallersCheck.Checked and HasInstallers;
    RemoveVersions := RemoveVersionsCheck.Checked and HasVersions;
    RemoveBackups := RemoveBackupsCheck.Checked and HasBackups;
  finally
    OptionsForm.Free;
  end;
end;

function InitializeUninstall(): Boolean;
begin
  Result := True;
  while IsAppRunning() do
  begin
    if MsgBox(ExpandConstant('{cm:MsgCloseBeforeUninstall}'), mbError, MB_RETRYCANCEL) = IDCANCEL then
    begin
      Result := False;
      exit;
    end;
  end;

  UninstallBaseRoot := ResolveBaseRootFromConfig();
  RemoveInstallers := False;
  RemoveVersions := False;
  RemoveBackups := False;

  if not IsSilentUninstall() then
  begin
    if HasOptionalBaseRootData(UninstallBaseRoot) then
    begin
      if not ShowUninstallOptions(UninstallBaseRoot) then
      begin
        Result := False;
        exit;
      end;
    end
    else
      Log(ExpandConstant('{cm:UninstallNoOptionalData}'));
  end;
end;

procedure DeleteDirIfExists(const DirPath: string);
begin
  if (DirPath <> '') and DirExists(DirPath) then
    DelTree(DirPath, True, True, True);
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  LocalPath: string;
  BaseRoot: string;
  CompanyDir: string;
begin
  if CurUninstallStep <> usUninstall then
    exit;

  LocalPath := ExpandConstant('{userappdata}\{#AppExeName}');
  BaseRoot := UninstallBaseRoot;

  if (BaseRoot <> '') and RemoveInstallers then
    DeleteDirIfExists(BaseRoot + '\installers');
  if (BaseRoot <> '') and RemoveVersions then
    DeleteDirIfExists(BaseRoot + '\versions');

  if (BaseRoot <> '') and RemoveBackups then
    DeleteDirIfExists(BaseRoot + '\backups');

  DeleteFile(LocalPath + '\config.json');
  DeleteFile(LocalPath + '\user_gamertag_map.json');
  DeleteDirIfExists(LocalPath + '\EBWebView');
  DeleteDirIfExists(LocalPath + '\bin');
  if DirExists(LocalPath) then
    RemoveDir(LocalPath);

  CompanyDir := ExpandConstant('{autopf64}\{#AppPublisher}');
  if DirExists(CompanyDir) then
    RemoveDir(CompanyDir);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssInstall then
  begin
    if ShouldCleanupLegacyNsisUninstaller() then
      DeleteFile(AddBackslash(WizardDirValue()) + 'uninstall.exe');
  end
  else if CurStep = ssPostInstall then
  begin
    RegDeleteKeyIncludingSubkeys(HKLM64, LegacyNsisUninstallKey);
    RegDeleteKeyIncludingSubkeys(HKCU, LegacyNsisUninstallKey);
    RegDeleteKeyIncludingSubkeys(HKLM64, LegacyNsisUninstallKeyAlt);
    RegDeleteKeyIncludingSubkeys(HKCU, LegacyNsisUninstallKeyAlt);
  end;
end;

procedure InitializeWizard;
begin
  CurrentInnoInstallDir := ResolveCurrentInnoInstallDir();
  LegacyNsisInstallDir := ResolveLegacyNsisInstallDir();
  if (Trim(CurrentInnoInstallDir) <> '') and not WizardSilent then
  begin
    MsgBox(
      FmtMessage(CustomMessage('UpgradeModeDesc'), [CurrentInnoInstallDir]),
      mbInformation,
      MB_OK
    );
  end
  else if (Trim(LegacyNsisInstallDir) <> '') and not WizardSilent then
  begin
    MsgBox(
      FmtMessage(CustomMessage('NsisMigrationModeDesc'), [LegacyNsisInstallDir]),
      mbInformation,
      MB_OK
    );
  end;
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := False;
  if (PageID = wpSelectDir) and ((Trim(CurrentInnoInstallDir) <> '') or (Trim(LegacyNsisInstallDir) <> '')) then
    Result := True;
end;
