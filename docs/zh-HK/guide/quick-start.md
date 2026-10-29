# 5 分鐘上手

如果您只想用最短路徑從下載走到開始遊戲，請先看這一頁。在任何分頁中選擇 **Windows** 或 **Android**，整套文檔都會跟隨您的選擇。

## 1. 安裝 LeviLauncher

=== "Windows"

    - 從 [GitHub Releases](https://github.com/LiteLDev/LeviLauncher/releases) 頁面或[藍奏雲鏡像](https://levimc.lanzoue.com/b016ke39hc) 下載 LeviLauncher 安裝程式，藍奏雲密碼為 `levi`。
    - 在 Windows 上執行安裝程式。
    - 如果安裝過程提示需要 WebView2 或其他依賴項，請允許其繼續安裝。

=== "Android"

    - 從 [GitHub Releases](https://github.com/LiteLDev/LeviLaunchroid/releases/latest) 下載最新 APK。
    - 允許您使用的瀏覽器或檔案管理器安裝應用程式，然後安裝 APK。
    - 打開 LeviLauncher，接受啟動器協議並授予它請求的權限。

## 2. 準備好 Minecraft

=== "Windows"

    - 登入擁有 **Minecraft Bedrock Edition** 的 Microsoft 帳號。
    - 至少在此電腦上從 Microsoft Store 安裝過一次遊戲。
    - 確保 **Gaming Services** 可用（GDK 版本需要）。

    ::: warning 必須擁有正版
    LeviLauncher 用於管理 Minecraft Bedrock (GDK / UWP)，並不能替代遊戲授權。
    :::

=== "Android"

    - 在本裝置上從 **Google Play** 安裝 **Minecraft Bedrock Edition** 1.21.80 或更高版本。
    - LeviLauncher 把這份官方遊戲作為可信的遊戲來源，它不能代替遊戲授權。

## 3. 選擇版本

=== "Windows"

    - 啟動 LeviLauncher。
    - 從側邊欄進入 **下載**。
    - 選擇 **正式版** 或 **預覽版**，以及 **GDK** 或 **UWP** 套件。
    - 這裡的 **正式版** 和 **預覽版** 分別指 Minecraft 的正式版（Release）與預覽版（Preview）。
    - 選擇版本並開始安裝。

=== "Android"

    - 進入版本（實例）頁面。
    - 選擇要執行的 Minecraft 版本，或匯入官方 Minecraft APK，在啟動器內執行其他版本。

## 4. 決定是否啟用隔離

=== "Windows"

    隔離功能會將該版本的遊戲數據存放於獨立的工作區，避免與預設的 AppData 環境混用。GDK 版本以及 1.19.70.2 或以上的 UWP x64 版本支援隔離。

    以下場景建議啟用隔離：

    - 測試模組，但不想影響主環境
    - 希望將 Minecraft 正式版與預覽版分開
    - 想為不同遊玩目的保留不同的世界和資源包

=== "Android"

    啟動匯入版本時，如果它的世界、資源包和設定要與 Google Play 安裝的版本分開，請啟用版本隔離。匯入版本通常需要啟用後才能啟動。

## 5. 啟動已安裝版本

=== "Windows"

    - 返回 **啟動** 頁面。
    - 選取剛安裝的版本。
    - 點擊 **啟動**。

    如果 LeviLauncher 報告缺少 GameInput 或 Gaming Services，請先按提示完成依賴項安裝。

=== "Android"

    - 回到啟動頁並點擊 **啟動**。
    - 如果啟動失敗，先從失敗頁面打開啟動日誌查看原因，再調整設定。

## 6. 繼續閱覽對應章節

- 需要準備環境：[系統要求與安裝](./requirements-installation)
- 需要了解首次進入應用程式時的情況：[首次啟動](./first-launch)
- 需要整理版本：[版本管理](./version-management)
- 需要管理世界、資源包或截圖：[內容管理](./content-management)
- 需要使用模組或整合來源：[模組與整合](./mods-integrations)
