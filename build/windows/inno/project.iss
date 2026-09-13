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
  #define VCRuntimeArch "x64"
#elif AppArch == "arm64"
  #define AllowedArchitectures "arm64"
  #define VCRuntimeArch "arm64"
#else
  #error "Unsupported AppArch. Use amd64 or arm64."
#endif

#define AppUrl "https://github.com/LiteLDev/LeviLauncher"
#define VCRuntimeFile "vc_redist." + VCRuntimeArch + ".exe"

[Setup]
AppId=org.levimc.launcher
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppUrl}
AppSupportURL={#AppUrl}/issues
AppUpdatesURL={#AppUrl}/releases
AppCopyright=(c) 2024-2026, LeviMC
; GPLv3 section 9 grants the right to run the program without accepting the
; license, so the text is shown for information instead of as an install gate.
InfoBeforeFile=..\..\..\COPYING
DefaultDirName={code:GetDefaultInstallDir|{autopf64}\{#AppPublisher}\{#AppName}}
DefaultGroupName={#AppName}
PrivilegesRequired=admin
OutputDir=..\..\..\bin
OutputBaseFilename={#AppName}-{#AppArch}-installer
SetupIconFile=..\icon.ico
UninstallDisplayIcon={app}\{#AppExeName}
UninstallDisplayName={#AppName}
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
WizardImageFile=wizard-image.bmp,wizard-image-2x.bmp,wizard-image-3x.bmp,wizard-image-4x.bmp
WizardSmallImageFile=wizard-small.png,wizard-small-2x.png,wizard-small-3x.png,wizard-small-4x.png
DisableWelcomePage=no
DisableDirPage=auto
DisableProgramGroupPage=yes
UsePreviousAppDir=no
UsePreviousGroup=yes
; Matches the launcher's single-instance mutex (main.go), so Setup and the
; uninstaller detect a running launcher and ask the user to close it.
AppMutex=Global\LeviLauncher_SingleInstance
SetupMutex=Global\LeviLauncherSetup
; The launcher is relaunched from [Run] as the original user, not by Restart
; Manager, which would restart it inside the elevated setup session.
RestartApplications=no
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
Name: "japanese"; MessagesFile: "compiler:Languages\Japanese.isl"
Name: "korean"; MessagesFile: "compiler:Languages\Korean.isl"
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "french"; MessagesFile: "compiler:Languages\French.isl"
Name: "german"; MessagesFile: "compiler:Languages\German.isl"
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"

[LangOptions]
simpchinese.DialogFontName=Microsoft YaHei UI
simpchinese.WelcomeFontName=Microsoft YaHei UI

[CustomMessages]
english.WelcomeDescription=Manage your Minecraft Bedrock versions, instances and mods.%n%nVersion %1 · %2%n%nSetup will check and install the required runtime components.
english.PreparingEnvironment=Checking your environment
english.PreparingComponents=Installing required components
english.PreparingLauncher=Preparing LeviLauncher
english.CheckingRuntimes=Checking Microsoft Visual C++ and WebView2 runtimes...
english.RuntimesReady=Your environment is ready. LeviLauncher will be installed next.
english.UninstallInstallersDetail=installers · Downloaded game installation packages.
english.UninstallBackupsDetail=backups · Backup archives of your instances.
english.UninstallVersionsDetail=versions · Includes worlds, mods and other instance data.
english.UninstallContinue=Continue uninstall
english.InstallVCRuntime=Installing Microsoft Visual C++ Runtime...
english.InstallWebView2=Installing WebView2 Runtime...
english.RuntimeRestartNotice=LeviLauncher has been installed.%n%nThe following components reported that Windows needs to restart to complete their setup:%n%1%n%nYou can restart your computer later.
english.RuntimeInstallFailed=Could not install %1 (error %2).
english.RuntimeInstallerBusy=Another installation is already running. Waiting for it to finish (up to %1 s)...
english.RuntimeInstallRetry=Wait for any other installer or Windows Update to finish, check your Internet connection, then click Retry.
english.RuntimeNotDetected=%1 is still unavailable after installation. Restart Windows if required, then run setup again.
english.UpgradeModeDesc=Upgrade mode: the existing installation in %1 will be upgraded in place.
english.UninstallOptionsTitle=Optional Data Cleanup
english.UninstallOptionsDesc=Your instance library is kept by default. Only the items selected below will be deleted.
english.UninstallDetectedBaseRoot=Instance library location:
english.UninstallSelectWhatToDelete=Select folders to delete:
english.UninstallDangerNote=Selected folders will be permanently deleted.
english.UninstallDeleteInstallers=Delete downloaded packages
english.UninstallDeleteVersions=[High risk] Delete game instances
english.UninstallDeleteBackups=Delete backups
english.UninstallNoOptionalData=No optional BaseRoot cleanup targets were found. Keeping all instance data.

simpchinese.WelcomeDescription=管理 Minecraft 基岩版的游戏版本、实例与模组。%n%n版本 %1 · %2%n%n安装程序将自动检查并安装所需的运行环境。
simpchinese.PreparingEnvironment=正在检查运行环境
simpchinese.PreparingComponents=正在安装所需组件
simpchinese.PreparingLauncher=正在准备安装 LeviLauncher
simpchinese.CheckingRuntimes=正在检查 Microsoft Visual C++ 和 WebView2 运行环境...
simpchinese.RuntimesReady=运行环境已就绪，即将安装 LeviLauncher。
simpchinese.UninstallInstallersDetail=installers · 已下载的游戏安装包。
simpchinese.UninstallBackupsDetail=backups · 游戏实例的备份文件。
simpchinese.UninstallVersionsDetail=versions · 包含存档、模组及其他实例数据。
simpchinese.UninstallContinue=继续卸载
simpchinese.InstallVCRuntime=正在安装 Microsoft Visual C++ 运行库...
simpchinese.InstallWebView2=正在安装 WebView2 运行时...
simpchinese.RuntimeRestartNotice=LeviLauncher 已安装完成。%n%n以下组件报告需要重启 Windows，以完成运行环境的安装或更新：%n%1%n%n你可以选择稍后重启电脑。
simpchinese.RuntimeInstallFailed=无法安装 %1（错误 %2）。
simpchinese.RuntimeInstallerBusy=另一个安装程序正在运行，正在等待它完成（最多 %1 秒）...
simpchinese.RuntimeInstallRetry=请等待其他安装程序或 Windows 更新结束，并检查网络连接，然后点击“重试”。
simpchinese.RuntimeNotDetected=安装后仍未检测到可用的 %1。如需重启，请重启 Windows 后重新运行安装程序。
simpchinese.UpgradeModeDesc=升级模式：将对 %1 中的现有安装执行原地升级。
simpchinese.UninstallOptionsTitle=可选数据清理
simpchinese.UninstallOptionsDesc=默认保留实例库中的数据。仅删除你在下方勾选的项目。
simpchinese.UninstallDetectedBaseRoot=实例库位置：
simpchinese.UninstallSelectWhatToDelete=请选择要删除的目录：
simpchinese.UninstallDangerNote=所选文件夹将被永久删除。
simpchinese.UninstallDeleteInstallers=删除已下载的安装包
simpchinese.UninstallDeleteVersions=[高风险] 删除游戏实例
simpchinese.UninstallDeleteBackups=删除备份文件
simpchinese.UninstallNoOptionalData=未发现可选的 BaseRoot 清理目标，实例数据将全部保留。

japanese.WelcomeDescription=Minecraft Bedrock のバージョン、インスタンス、Mod を管理します。%n%nバージョン %1 · %2%n%n必要なランタイムを確認し、インストールします。
japanese.PreparingEnvironment=実行環境を確認しています
japanese.PreparingComponents=必要なコンポーネントをインストールしています
japanese.PreparingLauncher=LeviLauncher のインストールを準備しています
japanese.CheckingRuntimes=Microsoft Visual C++ と WebView2 のランタイムを確認しています...
japanese.RuntimesReady=実行環境の準備ができました。続いて LeviLauncher をインストールします。
japanese.UninstallInstallersDetail=installers · ダウンロード済みのゲームインストールパッケージ。
japanese.UninstallBackupsDetail=backups · ゲームインスタンスのバックアップ。
japanese.UninstallVersionsDetail=versions · ワールド、Mod、その他のインスタンスデータを含みます。
japanese.UninstallContinue=アンインストールを続行
japanese.InstallVCRuntime=Microsoft Visual C++ ランタイムをインストールしています...
japanese.InstallWebView2=WebView2 ランタイムをインストールしています...
japanese.RuntimeRestartNotice=LeviLauncher のインストールが完了しました。%n%n次のコンポーネントは、セットアップを完了するために Windows の再起動を要求しています:%n%1%n%nコンピューターは後で再起動できます。
japanese.RuntimeInstallFailed=%1 をインストールできませんでした（エラー %2）。
japanese.RuntimeInstallerBusy=別のインストールが実行中です。完了を待っています（最大 %1 秒）...
japanese.RuntimeInstallRetry=他のインストーラーや Windows Update の完了を待ち、インターネット接続を確認してから「再試行」をクリックしてください。
japanese.RuntimeNotDetected=インストール後も %1 が見つかりません。必要に応じて Windows を再起動し、セットアップを再実行してください。
japanese.UpgradeModeDesc=アップグレードモード: %1 にある既存のインストールをその場でアップグレードします。
japanese.UninstallOptionsTitle=オプションのデータ削除
japanese.UninstallOptionsDesc=インスタンスライブラリのデータは標準で保持されます。下で選択した項目だけを削除します。
japanese.UninstallDetectedBaseRoot=インスタンスライブラリの場所:
japanese.UninstallSelectWhatToDelete=削除するフォルダを選択してください:
japanese.UninstallDangerNote=選択したフォルダーは完全に削除されます。
japanese.UninstallDeleteInstallers=ダウンロード済みのパッケージを削除
japanese.UninstallDeleteVersions=[危険] ゲームインスタンスを削除
japanese.UninstallDeleteBackups=バックアップを削除
japanese.UninstallNoOptionalData=削除対象の BaseRoot データが見つかりませんでした。インスタンスデータはすべて保持されます。

korean.WelcomeDescription=Minecraft Bedrock 버전, 인스턴스, 모드를 관리합니다.%n%n버전 %1 · %2%n%n설치 프로그램이 필요한 런타임을 확인하고 설치합니다.
korean.PreparingEnvironment=실행 환경 확인 중
korean.PreparingComponents=필수 구성 요소 설치 중
korean.PreparingLauncher=LeviLauncher 설치 준비 중
korean.CheckingRuntimes=Microsoft Visual C++ 및 WebView2 런타임을 확인하는 중...
korean.RuntimesReady=실행 환경이 준비되었습니다. 이제 LeviLauncher를 설치합니다.
korean.UninstallInstallersDetail=installers · 다운로드한 게임 설치 패키지.
korean.UninstallBackupsDetail=backups · 게임 인스턴스의 백업 파일.
korean.UninstallVersionsDetail=versions · 월드, 모드 및 기타 인스턴스 데이터가 포함됩니다.
korean.UninstallContinue=제거 계속
korean.InstallVCRuntime=Microsoft Visual C++ 런타임을 설치하는 중...
korean.InstallWebView2=WebView2 런타임을 설치하는 중...
korean.RuntimeRestartNotice=LeviLauncher 설치가 완료되었습니다.%n%n다음 구성 요소의 설치 또는 업데이트를 완료하려면 Windows를 다시 시작해야 합니다:%n%1%n%n컴퓨터를 나중에 다시 시작할 수 있습니다.
korean.RuntimeInstallFailed=%1을(를) 설치할 수 없습니다(오류 %2).
korean.RuntimeInstallerBusy=다른 설치가 진행 중입니다. 완료될 때까지 기다리는 중(최대 %1초)...
korean.RuntimeInstallRetry=다른 설치 프로그램이나 Windows 업데이트가 끝날 때까지 기다린 후 인터넷 연결을 확인하고 [다시 시도]를 클릭하십시오.
korean.RuntimeNotDetected=설치 후에도 %1을(를) 찾을 수 없습니다. 필요하면 Windows를 다시 시작한 다음 설치 프로그램을 다시 실행하십시오.
korean.UpgradeModeDesc=업그레이드 모드: %1에 있는 기존 설치를 그 자리에서 업그레이드합니다.
korean.UninstallOptionsTitle=선택적 데이터 정리
korean.UninstallOptionsDesc=인스턴스 라이브러리 데이터는 기본적으로 유지됩니다. 아래에서 선택한 항목만 삭제됩니다.
korean.UninstallDetectedBaseRoot=인스턴스 라이브러리 위치:
korean.UninstallSelectWhatToDelete=삭제할 폴더를 선택하십시오:
korean.UninstallDangerNote=선택한 폴더는 영구적으로 삭제됩니다.
korean.UninstallDeleteInstallers=다운로드한 패키지 삭제
korean.UninstallDeleteVersions=[높은 위험] 게임 인스턴스 삭제
korean.UninstallDeleteBackups=백업 삭제
korean.UninstallNoOptionalData=삭제할 수 있는 BaseRoot 데이터를 찾지 못했습니다. 인스턴스 데이터는 모두 유지됩니다.

russian.WelcomeDescription=Управляйте версиями Minecraft Bedrock, сборками игры и модами.%n%nВерсия %1 · %2%n%nПрограмма установки проверит и установит необходимые компоненты.
russian.PreparingEnvironment=Проверка среды выполнения
russian.PreparingComponents=Установка необходимых компонентов
russian.PreparingLauncher=Подготовка к установке LeviLauncher
russian.CheckingRuntimes=Проверка сред выполнения Microsoft Visual C++ и WebView2...
russian.RuntimesReady=Среда готова. Далее будет установлен LeviLauncher.
russian.UninstallInstallersDetail=installers · Загруженные установочные пакеты игры.
russian.UninstallBackupsDetail=backups · Резервные копии сборок игры.
russian.UninstallVersionsDetail=versions · Включает миры, моды и другие данные сборок.
russian.UninstallContinue=Продолжить удаление
russian.InstallVCRuntime=Установка среды выполнения Microsoft Visual C++...
russian.InstallWebView2=Установка среды выполнения WebView2...
russian.RuntimeRestartNotice=LeviLauncher установлен.%n%nДля завершения установки следующих компонентов требуется перезагрузка Windows:%n%1%n%nВы можете перезагрузить компьютер позже.
russian.RuntimeInstallFailed=Не удалось установить %1 (ошибка %2).
russian.RuntimeInstallerBusy=Уже выполняется другая установка. Ожидание её завершения (до %1 с)...
russian.RuntimeInstallRetry=Дождитесь завершения другой установки или Центра обновления Windows, проверьте подключение к Интернету и нажмите «Повтор».
russian.RuntimeNotDetected=Компонент %1 не обнаружен после установки. При необходимости перезагрузите Windows и запустите программу установки снова.
russian.UpgradeModeDesc=Режим обновления: существующая установка в %1 будет обновлена на месте.
russian.UninstallOptionsTitle=Дополнительная очистка данных
russian.UninstallOptionsDesc=Данные библиотеки сборок сохраняются по умолчанию. Будут удалены только выбранные ниже элементы.
russian.UninstallDetectedBaseRoot=Расположение библиотеки сборок:
russian.UninstallSelectWhatToDelete=Выберите папки для удаления:
russian.UninstallDangerNote=Выбранные папки будут удалены безвозвратно.
russian.UninstallDeleteInstallers=Удалить загруженные пакеты
russian.UninstallDeleteVersions=[Высокий риск] Удалить сборки игры
russian.UninstallDeleteBackups=Удалить резервные копии
russian.UninstallNoOptionalData=Данные BaseRoot для необязательной очистки не найдены. Все данные сборок сохранены.

french.WelcomeDescription=Gérez vos versions de Minecraft Bedrock, vos instances et vos mods.%n%nVersion %1 · %2%n%nLe programme vérifiera et installera les composants requis.
french.PreparingEnvironment=Vérification de l'environnement
french.PreparingComponents=Installation des composants requis
french.PreparingLauncher=Préparation de LeviLauncher
french.CheckingRuntimes=Vérification des runtimes Microsoft Visual C++ et WebView2...
french.RuntimesReady=L'environnement est prêt. LeviLauncher sera installé ensuite.
french.UninstallInstallersDetail=installers · Paquets d'installation du jeu téléchargés.
french.UninstallBackupsDetail=backups · Archives de sauvegarde de vos instances.
french.UninstallVersionsDetail=versions · Inclut les mondes, les mods et les autres données d'instance.
french.UninstallContinue=Continuer la désinstallation
french.InstallVCRuntime=Installation du runtime Microsoft Visual C++...
french.InstallWebView2=Installation du runtime WebView2...
french.RuntimeRestartNotice=LeviLauncher est installé.%n%nLes composants suivants demandent un redémarrage de Windows pour terminer leur installation :%n%1%n%nVous pouvez redémarrer votre ordinateur plus tard.
french.RuntimeInstallFailed=Impossible d'installer %1 (erreur %2).
french.RuntimeInstallerBusy=Une autre installation est déjà en cours. Attente de sa fin (jusqu'à %1 s)...
french.RuntimeInstallRetry=Attendez la fin de l'autre installation ou de Windows Update, vérifiez votre connexion Internet, puis cliquez sur Réessayer.
french.RuntimeNotDetected=%1 reste introuvable après l'installation. Redémarrez Windows si nécessaire, puis relancez le programme d'installation.
french.UpgradeModeDesc=Mode mise à niveau : l'installation existante dans %1 sera mise à niveau sur place.
french.UninstallOptionsTitle=Nettoyage facultatif des données
french.UninstallOptionsDesc=Votre bibliothèque d'instances est conservée par défaut. Seuls les éléments sélectionnés ci-dessous seront supprimés.
french.UninstallDetectedBaseRoot=Emplacement de la bibliothèque d'instances :
french.UninstallSelectWhatToDelete=Sélectionnez les dossiers à supprimer :
french.UninstallDangerNote=Les dossiers sélectionnés seront supprimés définitivement.
french.UninstallDeleteInstallers=Supprimer les paquets téléchargés
french.UninstallDeleteVersions=[Risque élevé] Supprimer les instances
french.UninstallDeleteBackups=Supprimer les sauvegardes
french.UninstallNoOptionalData=Aucune donnée BaseRoot facultative à nettoyer n'a été trouvée. Toutes les données d'instance sont conservées.

german.WelcomeDescription=Verwalten Sie Minecraft-Bedrock-Versionen, Spielinstanzen und Mods.%n%nVersion %1 · %2%n%nDas Setup prüft und installiert die erforderlichen Laufzeitkomponenten.
german.PreparingEnvironment=Laufzeitumgebung wird geprüft
german.PreparingComponents=Erforderliche Komponenten werden installiert
german.PreparingLauncher=LeviLauncher wird vorbereitet
german.CheckingRuntimes=Microsoft Visual C++ und WebView2 werden geprüft...
german.RuntimesReady=Die Umgebung ist bereit. Als Nächstes wird LeviLauncher installiert.
german.UninstallInstallersDetail=installers · Heruntergeladene Installationspakete des Spiels.
german.UninstallBackupsDetail=backups · Sicherungsarchive Ihrer Spielinstanzen.
german.UninstallVersionsDetail=versions · Enthält Welten, Mods und weitere Instanzdaten.
german.UninstallContinue=Deinstallation fortsetzen
german.InstallVCRuntime=Microsoft Visual C++ Runtime wird installiert...
german.InstallWebView2=WebView2-Runtime wird installiert...
german.RuntimeRestartNotice=LeviLauncher wurde installiert.%n%nDie folgenden Komponenten benötigen einen Windows-Neustart, um ihre Installation abzuschließen:%n%1%n%nSie können den Computer später neu starten.
german.RuntimeInstallFailed=%1 konnte nicht installiert werden (Fehler %2).
german.RuntimeInstallerBusy=Eine andere Installation läuft bereits. Es wird auf deren Abschluss gewartet (bis zu %1 s)...
german.RuntimeInstallRetry=Warten Sie, bis die andere Installation oder Windows Update abgeschlossen ist, prüfen Sie Ihre Internetverbindung und klicken Sie dann auf Wiederholen.
german.RuntimeNotDetected=%1 ist nach der Installation weiterhin nicht verfügbar. Starten Sie Windows bei Bedarf neu und führen Sie das Setup erneut aus.
german.UpgradeModeDesc=Upgrade-Modus: Die vorhandene Installation in %1 wird an Ort und Stelle aktualisiert.
german.UninstallOptionsTitle=Optionale Datenbereinigung
german.UninstallOptionsDesc=Ihre Instanzbibliothek bleibt standardmäßig erhalten. Nur die unten ausgewählten Elemente werden gelöscht.
german.UninstallDetectedBaseRoot=Speicherort der Instanzbibliothek:
german.UninstallSelectWhatToDelete=Zu löschende Ordner auswählen:
german.UninstallDangerNote=Die ausgewählten Ordner werden endgültig gelöscht.
german.UninstallDeleteInstallers=Heruntergeladene Pakete löschen
german.UninstallDeleteVersions=[Hohes Risiko] Spielinstanzen löschen
german.UninstallDeleteBackups=Sicherungen löschen
german.UninstallNoOptionalData=Es wurden keine optionalen BaseRoot-Daten zum Bereinigen gefunden. Alle Instanzdaten bleiben erhalten.

spanish.WelcomeDescription=Gestiona tus versiones de Minecraft Bedrock, instancias y mods.%n%nVersión %1 · %2%n%nEl instalador comprobará e instalará los componentes necesarios.
spanish.PreparingEnvironment=Comprobando el entorno
spanish.PreparingComponents=Instalando los componentes necesarios
spanish.PreparingLauncher=Preparando LeviLauncher
spanish.CheckingRuntimes=Comprobando los entornos de Microsoft Visual C++ y WebView2...
spanish.RuntimesReady=El entorno está listo. A continuación se instalará LeviLauncher.
spanish.UninstallInstallersDetail=installers · Paquetes de instalación del juego descargados.
spanish.UninstallBackupsDetail=backups · Copias de seguridad de tus instancias.
spanish.UninstallVersionsDetail=versions · Incluye mundos, mods y otros datos de las instancias.
spanish.UninstallContinue=Continuar desinstalación
spanish.InstallVCRuntime=Instalando Microsoft Visual C++ Runtime...
spanish.InstallWebView2=Instalando WebView2 Runtime...
spanish.RuntimeRestartNotice=LeviLauncher se ha instalado.%n%nLos siguientes componentes requieren reiniciar Windows para completar su instalación:%n%1%n%nPuedes reiniciar el equipo más tarde.
spanish.RuntimeInstallFailed=No se pudo instalar %1 (error %2).
spanish.RuntimeInstallerBusy=Ya hay otra instalación en curso. Esperando a que termine (hasta %1 s)...
spanish.RuntimeInstallRetry=Espere a que termine la otra instalación o Windows Update, compruebe su conexión a Internet y haga clic en Reintentar.
spanish.RuntimeNotDetected=%1 sigue sin estar disponible después de la instalación. Reinicie Windows si es necesario y vuelva a ejecutar el instalador.
spanish.UpgradeModeDesc=Modo de actualización: la instalación existente en %1 se actualizará en el mismo lugar.
spanish.UninstallOptionsTitle=Limpieza opcional de datos
spanish.UninstallOptionsDesc=Tu biblioteca de instancias se conserva por defecto. Solo se eliminarán los elementos seleccionados a continuación.
spanish.UninstallDetectedBaseRoot=Ubicación de la biblioteca de instancias:
spanish.UninstallSelectWhatToDelete=Seleccione las carpetas que desea eliminar:
spanish.UninstallDangerNote=Las carpetas seleccionadas se eliminarán de forma permanente.
spanish.UninstallDeleteInstallers=Eliminar paquetes descargados
spanish.UninstallDeleteVersions=[Riesgo alto] Eliminar instancias
spanish.UninstallDeleteBackups=Eliminar copias de seguridad
spanish.UninstallNoOptionalData=No se encontraron datos opcionales de BaseRoot para limpiar. Se conservan todos los datos de instancias.

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: checkedonce

[Files]
; Extracted to {tmp} from the Preparing page. Solid compression decompresses
; every preceding file in the stream, so these come before the payload.
Source: "{#VCRuntimeFile}"; Flags: dontcopy
Source: "MicrosoftEdgeWebview2Setup.exe"; Flags: dontcopy
Source: "{#AppBinaryPath}"; DestDir: "{app}"; DestName: "{#AppExeName}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Run]
; runasoriginaluser keeps the launcher out of the elevated setup session, so it
; reads and writes the signed-in user's AppData rather than the administrator's.
Filename: "{app}\{#AppExeName}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent runasoriginaluser

[Code]
const
  CurrentInnoUninstallKey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\org.levimc.launcher_is1';
  LegacyNsisUninstallKey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{#AppPublisher}{#AppName}';
  LegacyNsisUninstallKeyAlt = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{#AppName}';
  DialogMargin = 16;
  DangerColor = clMaroon;

var
  UninstallBaseRoot: string;
  RemoveInstallers: Boolean;
  RemoveVersions: Boolean;
  RemoveBackups: Boolean;
  CurrentInnoInstallDir: string;
  LegacyNsisInstallDir: string;

#include "prerequisites.iss"

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

function ExistingInstallDir: string;
begin
  Result := Trim(CurrentInnoInstallDir);
  if Result = '' then
    Result := Trim(LegacyNsisInstallDir);
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

function AddDialogLabel(OwnerForm: TSetupForm; const Caption: string; Bold: Boolean; Color: TColor; var CurTop: Integer): TNewStaticText;
begin
  Result := TNewStaticText.Create(OwnerForm);
  Result.Parent := OwnerForm;
  Result.Left := ScaleX(DialogMargin);
  Result.Top := CurTop;
  Result.Width := OwnerForm.ClientWidth - ScaleX(2 * DialogMargin);
  Result.AutoSize := False;
  Result.WordWrap := True;
  if Bold then
    Result.Font.Style := [fsBold];
  Result.Font.Color := Color;
  Result.Caption := Caption;
  Result.AdjustHeight;
  CurTop := Result.Top + Result.Height;
end;

function AddCleanupOption(OwnerForm: TSetupForm; const Caption, Description: string;
  DescriptionColor: TColor; var CurTop: Integer): TNewCheckBox;
var
  DetailLabel: TNewStaticText;
begin
  Result := TNewCheckBox.Create(OwnerForm);
  Result.Parent := OwnerForm;
  Result.Left := ScaleX(DialogMargin);
  Result.Top := CurTop;
  Result.Width := OwnerForm.ClientWidth - ScaleX(2 * DialogMargin);
  Result.Height := ScaleY(21);
  Result.Caption := Caption;
  Result.Font.Style := [fsBold];
  Result.Checked := False;
  CurTop := Result.Top + Result.Height;
  DetailLabel := AddDialogLabel(OwnerForm, Description, False, DescriptionColor, CurTop);
  DetailLabel.Left := Result.Left + ScaleX(20);
  DetailLabel.Width := Result.Width - ScaleX(20);
  DetailLabel.AdjustHeight;
  CurTop := DetailLabel.Top + DetailLabel.Height + ScaleY(12);
end;

function ShowUninstallOptions(const BaseRoot: string): Boolean;
var
  OptionsForm: TSetupForm;
  BaseRootPathEdit: TNewEdit;
  InstallersCheck: TNewCheckBox;
  VersionsCheck: TNewCheckBox;
  BackupsCheck: TNewCheckBox;
  OkButton: TNewButton;
  CancelButton: TNewButton;
  CurTop: Integer;
  ButtonWidth: Integer;
begin
  Result := True;
  RemoveInstallers := False;
  RemoveVersions := False;
  RemoveBackups := False;
  InstallersCheck := nil;
  VersionsCheck := nil;
  BackupsCheck := nil;

  // CreateCustomForm initializes the font and scales its client size itself.
  // Pass design units once; keep the dialog independent of WizardSizePercent.
  OptionsForm := CreateCustomForm(460, 200, True, True);
  try
    OptionsForm.Caption := ExpandConstant('{cm:UninstallOptionsTitle}');

    CurTop := ScaleY(DialogMargin);
    AddDialogLabel(OptionsForm, ExpandConstant('{cm:UninstallOptionsTitle}'), True, clWindowText, CurTop);
    CurTop := CurTop + ScaleY(6);
    AddDialogLabel(OptionsForm, ExpandConstant('{cm:UninstallOptionsDesc}'), False, clWindowText, CurTop);
    CurTop := CurTop + ScaleY(12);

    AddDialogLabel(OptionsForm, ExpandConstant('{cm:UninstallDetectedBaseRoot}'), False, clWindowText, CurTop);
    CurTop := CurTop + ScaleY(3);

    BaseRootPathEdit := TNewEdit.Create(OptionsForm);
    BaseRootPathEdit.Parent := OptionsForm;
    BaseRootPathEdit.Left := ScaleX(DialogMargin);
    BaseRootPathEdit.Top := CurTop;
    BaseRootPathEdit.Width := OptionsForm.ClientWidth - ScaleX(2 * DialogMargin);
    BaseRootPathEdit.Height := ScaleY(23);
    BaseRootPathEdit.ReadOnly := True;
    BaseRootPathEdit.Text := BaseRoot;
    CurTop := BaseRootPathEdit.Top + BaseRootPathEdit.Height + ScaleY(14);

    AddDialogLabel(OptionsForm, ExpandConstant('{cm:UninstallSelectWhatToDelete}'), True, clWindowText, CurTop);
    CurTop := CurTop + ScaleY(4);
    if DirExists(BaseRoot + '\installers') then
      InstallersCheck := AddCleanupOption(OptionsForm, ExpandConstant('{cm:UninstallDeleteInstallers}'),
        ExpandConstant('{cm:UninstallInstallersDetail}'), clWindowText, CurTop);
    if DirExists(BaseRoot + '\backups') then
      BackupsCheck := AddCleanupOption(OptionsForm, ExpandConstant('{cm:UninstallDeleteBackups}'),
        ExpandConstant('{cm:UninstallBackupsDetail}'), clWindowText, CurTop);
    if DirExists(BaseRoot + '\versions') then
      VersionsCheck := AddCleanupOption(OptionsForm, ExpandConstant('{cm:UninstallDeleteVersions}'),
        ExpandConstant('{cm:UninstallVersionsDetail}'), DangerColor, CurTop);

    AddDialogLabel(OptionsForm, ExpandConstant('{cm:UninstallDangerNote}'), False, DangerColor, CurTop);
    CurTop := CurTop + ScaleY(16);
    OptionsForm.ClientHeight := CurTop + ScaleY(23 + DialogMargin);

    OkButton := TNewButton.Create(OptionsForm);
    OkButton.Parent := OptionsForm;
    OkButton.Caption := ExpandConstant('{cm:UninstallContinue}');
    OkButton.ModalResult := mrOk;
    OkButton.Default := True;
    OkButton.Height := ScaleY(23);
    OkButton.Top := CurTop;

    CancelButton := TNewButton.Create(OptionsForm);
    CancelButton.Parent := OptionsForm;
    CancelButton.Caption := SetupMessage(msgButtonCancel);
    CancelButton.ModalResult := mrCancel;
    CancelButton.Cancel := True;
    CancelButton.Height := ScaleY(23);
    CancelButton.Top := CurTop;

    ButtonWidth := OptionsForm.CalculateButtonWidth([OkButton.Caption, CancelButton.Caption]);
    OkButton.Width := ButtonWidth;
    CancelButton.Width := ButtonWidth;
    CancelButton.Left := OptionsForm.ClientWidth - ScaleX(DialogMargin) - ButtonWidth;
    OkButton.Left := CancelButton.Left - ScaleX(8) - ButtonWidth;

    OptionsForm.ActiveControl := CancelButton;

    if OptionsForm.ShowModal <> mrOk then
    begin
      Result := False;
      exit;
    end;

    RemoveInstallers := (InstallersCheck <> nil) and InstallersCheck.Checked;
    RemoveVersions := (VersionsCheck <> nil) and VersionsCheck.Checked;
    RemoveBackups := (BackupsCheck <> nil) and BackupsCheck.Checked;
  finally
    OptionsForm.Free;
  end;
end;

function InitializeUninstall(): Boolean;
begin
  Result := True;
  UninstallBaseRoot := ResolveBaseRootFromConfig();
  RemoveInstallers := False;
  RemoveVersions := False;
  RemoveBackups := False;

  if IsSilentUninstall() then
    exit;

  if not HasOptionalBaseRootData(UninstallBaseRoot) then
  begin
    Log(ExpandConstant('{cm:UninstallNoOptionalData}'));
    exit;
  end;

  Result := ShowUninstallOptions(UninstallBaseRoot);
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
  PublisherDir: string;
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

  // RemoveDir only succeeds on an empty directory, and the name check keeps the
  // cleanup to the publisher folder this installer creates around {app}.
  PublisherDir := ExtractFileDir(ExpandConstant('{app}'));
  if CompareText(ExtractFileName(PublisherDir), '{#AppPublisher}') = 0 then
    RemoveDir(PublisherDir);
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
  WizardForm.WelcomeLabel2.Caption := FmtMessage(CustomMessage('WelcomeDescription'), ['{#AppVersion}', '{#AppArch}']);
  InitializePreparingProgress;
end;

procedure CurPageChanged(CurPageID: Integer);
begin
  if CurPageID = wpFinished then
    ShowRuntimeRestartNotice;
end;

function ShouldSkipPage(PageID: Integer): Boolean;
begin
  Result := ((PageID = wpWelcome) or (PageID = wpSelectDir)) and (ExistingInstallDir <> '');
end;

function UpdateReadyMemo(const Space, NewLine, MemoUserInfoInfo, MemoDirInfo, MemoTypeInfo,
  MemoComponentsInfo, MemoGroupInfo, MemoTasksInfo: string): string;
begin
  Result := '';
  if ExistingInstallDir <> '' then
    Result := FmtMessage(CustomMessage('UpgradeModeDesc'), [ExistingInstallDir]) + NewLine + NewLine;
  Result := Result + MemoDirInfo;
  if MemoTasksInfo <> '' then
    Result := Result + NewLine + NewLine + MemoTasksInfo;
end;
