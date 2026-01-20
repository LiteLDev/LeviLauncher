Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows you to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
## 
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the wails_tools.nsh file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "my-project" # Default "myfirstapp3"
## !define INFO_COMPANYNAME    "My Company" # Default "My Company"
## !define INFO_PRODUCTNAME    "My Product Name" # Default "My Product"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "0.1.0"
## !define INFO_COPYRIGHT      "(c) Now, My Company" # Default "© now, My Company"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"
!include "nsDialogs.nsh"
!include "LogicLib.nsh"
!include "TextFunc.nsh"
!include "FileFunc.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!define MUI_PAGE_CUSTOMFUNCTION_PRE WelcomePre
!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!define MUI_PAGE_CUSTOMFUNCTION_PRE DirPre
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_CONFIRM
UninstPage custom un.customPage un.customPageLeave
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}" # Default installing folder ($PROGRAMFILES is Program Files folder).
InstallDirRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${UNINST_KEY_NAME}" "InstallLocation"
ShowInstDetails show # This will always show the installation details.
ShowUninstDetails show

Var BaseRoot
Var LogFileHandle
Var RoamingPath
Var LocalPath
Var ConfigPath
Var UninstallCleanBaseRootCheckbox
Var UninstallCleanBackupCheckbox
Var UninstallCleanBaseRootFlag
Var UninstallCleanBackupFlag
Var BaseRootLabel
Var ResolveBaseRootButton
Var InstallParent
Var IsUpgrade

LangString LBL_Roaming ${LANG_ENGLISH} "Roaming: $RoamingPath"
LangString LBL_Roaming ${LANG_SIMPCHINESE} "Roaming：$RoamingPath"
LangString CHK_RemoveRoaming ${LANG_ENGLISH} "Remove Roaming data"
LangString CHK_RemoveRoaming ${LANG_SIMPCHINESE} "删除漫游数据"
LangString LBL_Local ${LANG_ENGLISH} "Roaming: $LocalPath"
LangString LBL_Local ${LANG_SIMPCHINESE} "漫游：$LocalPath"
LangString CHK_RemoveLocal ${LANG_ENGLISH} "Remove Roaming data (config.json + bin)"
LangString CHK_RemoveLocal ${LANG_SIMPCHINESE} "删除漫游数据（config.json + bin）"
LangString LBL_BaseRoot ${LANG_ENGLISH} "BaseRoot: $BaseRoot"
LangString LBL_BaseRoot ${LANG_SIMPCHINESE} "BaseRoot：$BaseRoot"
LangString BTN_ResolveBaseRoot ${LANG_ENGLISH} "Resolve BaseRoot"
LangString BTN_ResolveBaseRoot ${LANG_SIMPCHINESE} "解析 BaseRoot"
LangString TIP_ResolveFirst ${LANG_ENGLISH} "Tip: Click 'Resolve BaseRoot' first if you want to delete data."
LangString TIP_ResolveFirst ${LANG_SIMPCHINESE} "提示：如需删除数据，请先点击上方的“解析 BaseRoot”按钮。"
LangString CHK_RemoveBaseRoot ${LANG_ENGLISH} "Remove installers and versions"
LangString CHK_RemoveBaseRoot ${LANG_SIMPCHINESE} "删除 installers 和 versions"
LangString CHK_RemoveBackup ${LANG_ENGLISH} "Remove backups (save backups)"
LangString CHK_RemoveBackup ${LANG_SIMPCHINESE} "删除备份（存档备份）"
LangString MSG_ConfirmBaseRoot ${LANG_ENGLISH} "Removing installers and versions under BaseRoot will delete downloaded versions and installers. Continue?"
LangString MSG_ConfirmBaseRoot ${LANG_SIMPCHINESE} "删除 BaseRoot 下的 installers 和 versions 将删除已下载的版本与安装包。是否继续？"
LangString MSG_ConfirmBackup ${LANG_ENGLISH} "Removing Backup will delete all backups under BaseRoot. Continue?"
LangString MSG_ConfirmBackup ${LANG_SIMPCHINESE} "删除备份将移除 BaseRoot 下的所有备份。是否继续？"
LangString TXT_UninstallInit ${LANG_ENGLISH} "Uninstall init"
LangString TXT_UninstallInit ${LANG_SIMPCHINESE} "卸载初始化"
LangString TXT_Blocked ${LANG_ENGLISH} "Blocked: application is running"
LangString TXT_Blocked ${LANG_SIMPCHINESE} "阻止：应用正在运行"
LangString MSG_CloseBeforeUninstall ${LANG_ENGLISH} "${INFO_PRODUCTNAME} is currently running. Please close it before uninstalling."
LangString MSG_CloseBeforeUninstall ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME} 正在运行。请关闭后再卸载。"
LangString TXT_UninstallStart ${LANG_ENGLISH} "Uninstall section start"
LangString TXT_UninstallStart ${LANG_SIMPCHINESE} "开始卸载"
LangString TXT_RoamingPath ${LANG_ENGLISH} "Roaming path: $RoamingPath"
LangString TXT_RoamingPath ${LANG_SIMPCHINESE} "漫游路径：$RoamingPath"
LangString TXT_LocalPath ${LANG_ENGLISH} "Roaming path: $LocalPath"
LangString TXT_LocalPath ${LANG_SIMPCHINESE} "漫游路径：$LocalPath"
LangString TXT_ConfigPath ${LANG_ENGLISH} "Config path: $ConfigPath"
LangString TXT_ConfigPath ${LANG_SIMPCHINESE} "配置路径：$ConfigPath"
LangString TXT_BaseRoot ${LANG_ENGLISH} "BaseRoot: $BaseRoot"
LangString TXT_BaseRoot ${LANG_SIMPCHINESE} "BaseRoot：$BaseRoot"
LangString TXT_RemoveRoaming ${LANG_ENGLISH} "Remove Roaming"
LangString TXT_RemoveRoaming ${LANG_SIMPCHINESE} "删除启动器数据"
LangString TXT_RoamingErr ${LANG_ENGLISH} "Roaming removal error"
LangString TXT_RoamingErr ${LANG_SIMPCHINESE} "删除启动器数据出错"
LangString TXT_RemoveBaseSubs ${LANG_ENGLISH} "Remove BaseRoot subdirectories: installers, versions"
LangString TXT_RemoveBaseSubs ${LANG_SIMPCHINESE} "删除 BaseRoot 子目录：installers、versions"
LangString TXT_InstallersErr ${LANG_ENGLISH} "Installers removal error"
LangString TXT_InstallersErr ${LANG_SIMPCHINESE} "删除 installers 出错"
LangString TXT_VersionsErr ${LANG_ENGLISH} "Versions removal error"
LangString TXT_VersionsErr ${LANG_SIMPCHINESE} "删除 versions 出错"
LangString TXT_RemoveBackup ${LANG_ENGLISH} "Remove Backup: $BaseRoot\\backups"
LangString TXT_RemoveBackup ${LANG_SIMPCHINESE} "删除备份：$BaseRoot\\backups"
LangString TXT_BackupErr ${LANG_ENGLISH} "Backup removal error"
LangString TXT_BackupErr ${LANG_SIMPCHINESE} "删除备份出错"
LangString TXT_RemoveLocalMatch ${LANG_ENGLISH} "Remove Roaming directory (matches BaseRoot)"
LangString TXT_RemoveLocalMatch ${LANG_SIMPCHINESE} "删除漫游目录（与 BaseRoot 相同）"
LangString TXT_LocalDirErr ${LANG_ENGLISH} "Roaming directory removal error"
LangString TXT_LocalDirErr ${LANG_SIMPCHINESE} "删除漫游目录出错"
LangString TXT_RemoveLocalParts ${LANG_ENGLISH} "Remove Roaming parts (config.json + bin)"
LangString TXT_RemoveLocalParts ${LANG_SIMPCHINESE} "删除漫游部分（config.json + bin）"
LangString TXT_ConfigErr ${LANG_ENGLISH} "Delete config.json error"
LangString TXT_ConfigErr ${LANG_SIMPCHINESE} "删除 config.json 出错"
LangString TXT_BinErr ${LANG_ENGLISH} "Remove bin error"
LangString TXT_BinErr ${LANG_SIMPCHINESE} "删除 bin 出错"
LangString TXT_RemoveInstDir ${LANG_ENGLISH} "InstallDir removal error"
LangString TXT_RemoveInstDir ${LANG_SIMPCHINESE} "删除安装目录出错"
LangString TXT_RemovedInstDir ${LANG_ENGLISH} "Removed install dir"
LangString TXT_RemovedInstDir ${LANG_SIMPCHINESE} "已删除安装目录"
LangString TXT_UninstallEnd ${LANG_ENGLISH} "Uninstall end"
LangString TXT_UninstallEnd ${LANG_SIMPCHINESE} "卸载结束"
LangString TXT_RemovedCompanyDir ${LANG_ENGLISH} "Removed company dir"
LangString TXT_RemovedCompanyDir ${LANG_SIMPCHINESE} "Removed company dir"
LangString TXT_CompanyDirNotRemoved ${LANG_ENGLISH} "Company dir not removed (not empty)"
LangString TXT_CompanyDirNotRemoved ${LANG_SIMPCHINESE} "Company dir not removed (not empty)"
LangString TXT_UpgradeBrand ${LANG_ENGLISH} "Upgrade mode detected"
LangString TXT_UpgradeBrand ${LANG_SIMPCHINESE} "检测到已安装，正在进行升级"

Function .onInit
   !insertmacro wails.checkArchitecture
   SetRegView 64
   StrCpy $IsUpgrade 0
   ReadRegStr $0 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${UNINST_KEY_NAME}" "InstallLocation"
   StrCmp $0 "" tryHKCU
     StrCpy $INSTDIR $0
     StrCpy $IsUpgrade 1
     Goto endInitReg
  tryHKCU:
    ReadRegStr $0 HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${UNINST_KEY_NAME}" "InstallLocation"
    StrCmp $0 "" tryHKLMIcon
    StrCpy $INSTDIR $0
    StrCpy $IsUpgrade 1
    Goto endInitReg
  tryHKLMIcon:
    ReadRegStr $1 HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${UNINST_KEY_NAME}" "DisplayIcon"
    StrCmp $1 "" tryHKCUIcon
    ${GetParent} "$1" $0
    StrCmp $0 "" endInitReg
    StrCpy $INSTDIR $0
    StrCpy $IsUpgrade 1
    Goto endInitReg
  tryHKCUIcon:
    ReadRegStr $1 HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\${UNINST_KEY_NAME}" "DisplayIcon"
    StrCmp $1 "" endInitReg
    ${GetParent} "$1" $0
    StrCmp $0 "" endInitReg
    StrCpy $INSTDIR $0
    StrCpy $IsUpgrade 1
  endInitReg:
FunctionEnd

Function DirPre
  StrCmp $IsUpgrade 1 skipDir
  Return
  skipDir:
    Abort
FunctionEnd

Function WelcomePre
  StrCmp $IsUpgrade 1 +2
  Return
  !insertmacro MUI_HEADER_TEXT "${INFO_PRODUCTNAME}" "$(TXT_UpgradeBrand)"
FunctionEnd

Function un.customPage
  SetShellVarContext current
  StrCpy $RoamingPath "$APPDATA\${PRODUCT_EXECUTABLE}"
  StrCpy $LocalPath "$APPDATA\${PRODUCT_EXECUTABLE}"
  StrCpy $ConfigPath "$APPDATA\${PRODUCT_EXECUTABLE}\config.json"
  StrCpy $BaseRoot ""

  nsDialogs::Create 1018
  Pop $0
  
  ${NSD_CreateLabel} 0u 56u 100% 10u "$(LBL_BaseRoot)"
  Pop $BaseRootLabel
  ${NSD_CreateButton} 0u 68u 40% 12u "$(BTN_ResolveBaseRoot)"
  Pop $ResolveBaseRootButton
  ${NSD_OnClick} $ResolveBaseRootButton un.resolveBaseRoot
  ${NSD_CreateLabel} 0u 84u 100% 10u "$(TIP_ResolveFirst)"
  ${NSD_CreateCheckbox} 0u 98u 100% 12u "$(CHK_RemoveBaseRoot)"
  Pop $UninstallCleanBaseRootCheckbox
  ${NSD_OnClick} $UninstallCleanBaseRootCheckbox un.onBaseRootCheckboxClick
  StrCmp $BaseRoot "" 0 +2
    EnableWindow $UninstallCleanBaseRootCheckbox 0
  ${NSD_CreateCheckbox} 0u 114u 100% 12u "$(CHK_RemoveBackup)"
  Pop $UninstallCleanBackupCheckbox
  ${NSD_OnClick} $UninstallCleanBackupCheckbox un.onBackupCheckboxClick
  StrCmp $BaseRoot "" 0 +2
    EnableWindow $UninstallCleanBackupCheckbox 0
  nsDialogs::Show
FunctionEnd

Function un.resolveBaseRoot
  SetShellVarContext current
  StrCpy $ConfigPath "$APPDATA\${PRODUCT_EXECUTABLE}\config.json"
  StrCpy $BaseRoot ""
  IfFileExists "$ConfigPath" 0 endResolve
    InitPluginsDir
    FileOpen $1 "$pluginsdir\read-base-root.ps1" w
    FileWrite $1 "$$path = Join-Path $$env:APPDATA '${PRODUCT_EXECUTABLE}\\config.json'$\r$\n"
    FileWrite $1 "if (Test-Path -LiteralPath $$path) {$\r$\n"
    FileWrite $1 "  try { $$cfg = Get-Content -Raw -Encoding UTF8 $$path | ConvertFrom-Json; [Console]::Write(([string]$$cfg.base_root).Trim()) } catch { }$\r$\n"
    FileWrite $1 "}$\r$\n"
    FileClose $1
    nsExec::ExecToStack 'powershell -NoProfile -ExecutionPolicy Bypass -File "$pluginsdir\read-base-root.ps1"'
    Pop $0
    Pop $BaseRoot
  ${NSD_SetText} $BaseRootLabel "$(LBL_BaseRoot)"
  StrCmp $BaseRoot "" 0 enableBase
    EnableWindow $UninstallCleanBaseRootCheckbox 0
    EnableWindow $UninstallCleanBackupCheckbox 0
    Goto endResolve
  enableBase:
    EnableWindow $UninstallCleanBaseRootCheckbox 1
    EnableWindow $UninstallCleanBackupCheckbox 1
  endResolve:
FunctionEnd

Function un.onBaseRootCheckboxClick
  ${NSD_GetState} $UninstallCleanBaseRootCheckbox $UninstallCleanBaseRootFlag
  ${If} $UninstallCleanBaseRootFlag == 1
    MessageBox MB_ICONEXCLAMATION|MB_YESNO "$(MSG_ConfirmBaseRoot)" IDYES endWarn IDNO uncheckBase
    uncheckBase:
      ${NSD_SetState} $UninstallCleanBaseRootCheckbox 0
      StrCpy $UninstallCleanBaseRootFlag 0
    endWarn:
  ${EndIf}
FunctionEnd

Function un.onBackupCheckboxClick
  ${NSD_GetState} $UninstallCleanBackupCheckbox $UninstallCleanBackupFlag
  ${If} $UninstallCleanBackupFlag == 1
    MessageBox MB_ICONEXCLAMATION|MB_YESNO "$(MSG_ConfirmBackup)" IDYES endWarn IDNO uncheckBackup
    uncheckBackup:
      ${NSD_SetState} $UninstallCleanBackupCheckbox 0
      StrCpy $UninstallCleanBackupFlag 0
    endWarn:
  ${EndIf}
FunctionEnd

Function un.RemoveDirForce
  Exch $0
  StrLen $1 $0
  IntCmp $1 0 end 0 0
  IntCmp $1 3 end 0 0
  IfFileExists "$0" 0 end
  IfFileExists "$0\*" 0 tryremove
  tryremove:
  FindFirst $2 $3 "$0\*"
  loop:
  StrCmp $3 "" endloop
  StrCmp $3 "." next
  StrCmp $3 ".." next
  IfFileExists "$0\$3\*" 0 isfile
  Push "$0\$3"
  Call un.RemoveDirForce
  Goto next
  isfile:
  SetFileAttributes "$0\$3" NORMAL
  Delete /REBOOTOK "$0\$3"
  next:
  FindNext $2 $3
  Goto loop
  endloop:
  FindClose $2
  RMDir "$0"
  end:
  Pop $0
FunctionEnd

Function un.FastRemoveDir
  Exch $0
  IfFileExists "$0" 0 end
  nsExec::Exec 'cmd /C rmdir /S /Q "$0"'
  IfFileExists "$0" 0 end
  Push "$0"
  Call un.RemoveDirForce
  end:
  Pop $0
FunctionEnd

Function un.customPageLeave
  ${NSD_GetState} $UninstallCleanBaseRootCheckbox $UninstallCleanBaseRootFlag
  ${NSD_GetState} $UninstallCleanBackupCheckbox $UninstallCleanBackupFlag
FunctionEnd

Function un.onInit
  SetDetailsPrint both
  DetailPrint "$(TXT_UninstallInit)"
  IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 done
  loop:
  System::Call 'kernel32::CreateFileW(w "$INSTDIR\${PRODUCT_EXECUTABLE}", i 0xC0000000, i 0, p 0, i 3, i 0x80, p 0) i .r0'
  StrCmp $0 -1 blocked ok
  System::Call 'kernel32::CloseHandle(i $0)'
  Goto done
  blocked:
  DetailPrint "$(TXT_Blocked)"
  MessageBox MB_ICONEXCLAMATION|MB_RETRYCANCEL "$(MSG_CloseBeforeUninstall)" IDRETRY loop
  Abort
  ok:
  System::Call 'kernel32::CloseHandle(i $0)'
  done:
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR
    SetOverwrite on
    
    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols
    
    !insertmacro wails.writeUninstaller
    SetRegView 64
    WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
    IfErrors 0 +2
    WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
SectionEnd

Section "uninstall" 
    !insertmacro wails.setShellContext

    SetDetailsPrint both
    DetailPrint "$(TXT_UninstallStart)"
    FileOpen $LogFileHandle "$TEMP\${INFO_PRODUCTNAME}-uninstall.log" w
    FileWrite $LogFileHandle "Uninstall start\r\n"

    SetShellVarContext current
    DetailPrint "$(TXT_RoamingPath)"
    DetailPrint "$(TXT_LocalPath)"
    DetailPrint "$(TXT_ConfigPath)"
    DetailPrint "$(TXT_BaseRoot)"
    FileWrite $LogFileHandle "Roaming: $RoamingPath\r\n"
    FileWrite $LogFileHandle "Local: $LocalPath\r\n"
    FileWrite $LogFileHandle "Config: $ConfigPath\r\n"
    FileWrite $LogFileHandle "BaseRoot: $BaseRoot\r\n"

    

    ${If} $UninstallCleanBaseRootFlag == 1
        DetailPrint "$(TXT_RemoveBaseSubs)"
        Push "$BaseRoot\installers"
        Call un.FastRemoveDir
        IfFileExists "$BaseRoot\installers" 0 +2
        DetailPrint "$(TXT_InstallersErr)"
        Push "$BaseRoot\versions"
        Call un.FastRemoveDir
        IfFileExists "$BaseRoot\versions" 0 +2
        DetailPrint "$(TXT_VersionsErr)"
    ${EndIf}

    ${If} $UninstallCleanBackupFlag == 1
        DetailPrint "$(TXT_RemoveBackup)"
        Push "$BaseRoot\backups"
        Call un.FastRemoveDir
        IfFileExists "$BaseRoot\backups" 0 +2
        DetailPrint "$(TXT_BackupErr)"
    ${EndIf}
    
    
    DetailPrint "$(TXT_RemoveLocalParts)"
    Delete "$LocalPath\config.json"
    Delete "$LocalPath\user_gamertag_map.json"
    IfErrors 0 +2
    DetailPrint "$(TXT_ConfigErr)"
    Push "$LocalPath\EBWebView"
    Call un.FastRemoveDir
    Push "$LocalPath\bin"
    Call un.FastRemoveDir
    IfFileExists "$LocalPath\bin" 0 +2
    DetailPrint "$(TXT_BinErr)"
    
    RMDir "$LocalPath"

    SetShellVarContext all

    RMDir /r $INSTDIR
    IfErrors 0 +2
    DetailPrint "$(TXT_RemoveInstDir)"
    FileWrite $LogFileHandle "$(TXT_RemovedInstDir)\r\n"

    ${GetParent} "$INSTDIR" $InstallParent
    StrCmp $InstallParent "" doneParent
    StrCmp $InstallParent "$PROGRAMFILES64\${INFO_COMPANYNAME}" 0 doneParent
    RMDir $InstallParent
    IfErrors 0 +2
    DetailPrint "$(TXT_CompanyDirNotRemoved)"
    IfFileExists "$InstallParent" 0 +2
    FileWrite $LogFileHandle "$(TXT_RemovedCompanyDir)\r\n"
    doneParent:

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    FileWrite $LogFileHandle "$(TXT_UninstallEnd)\r\n"
    FileClose $LogFileHandle
    !insertmacro wails.deleteUninstaller
SectionEnd
