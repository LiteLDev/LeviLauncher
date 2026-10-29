# 首次啟動

第一次打開 LeviLauncher 時，應用程式會先確認您的裝置與 Minecraft 已經準備好進行託管版本管理。

## 你可能會看到什麼

=== "Windows"

    首次啟動時，LeviLauncher 可能會：

    - 檢查 Gaming Services 是否可用（GDK）
    - 檢查 GameInput 是否可用（GDK）
    - 檢查原生模組載入器所需的 Visual C++ 執行庫是否已安裝
    - 驗證 Minecraft Bedrock (GDK / UWP) 環境是否已準備就緒
    - 在缺失關鍵組件時顯示引導或提示

=== "Android"

    首次啟動時，LeviLauncher 可能會：

    - 請您接受啟動器協議
    - 請求所需權限
    - 檢查是否安裝了 Google Play 官方 Minecraft（1.21.80 或更高版本）
    - 遷移舊版啟動器留下的資料目錄

## 常見首次啟動流程

=== "Windows"

    1. 開啟 LeviLauncher。
    2. 閱讀啟動器給出的引導資訊。
    3. 安裝或修復缺失的前置組件。
    4. 返回啟動器重新檢查狀態。
    5. 滿足條件後進入 **下載** 頁面。

=== "Android"

    1. 打開 LeviLauncher。
    2. 接受協議並授予請求的權限。
    3. 如果啟動器正在遷移舊資料，請等待完成。在確認世界、資源包和設定都可用之前，不要刪除舊目錄。
    4. 進入版本頁面。

## 如果提示缺少組件 <Badge platform="windows" />

### Gaming Services

如果 Gaming Services 缺失或損壞，LeviLauncher 可能會引導你前往 Microsoft Store 進行安裝或修復。

### GameInput

如果缺少 GameInput，請按提示安裝所需的 Redistributable 套件。

### 未偵測到 Minecraft Bedrock

請確認：

- 當前 Microsoft 帳號擁有遊戲授權
- 當前電腦上已經從 Microsoft Store 安裝過遊戲
- 你並非試圖繞過官方授權流程使用啟動器

## 給新用戶的良好預設值

- 先使用一個隔離的正式版（Release）版本
- 將下載與內容路徑設定在可寫入的磁碟位置
- 在第一次成功純淨啟動之前，不要急於匯入大量模組

## 首次啟動之後建議繼續閱覽

- [版本管理](./version-management)
- [內容管理](./content-management)
- [設定與個人化](./settings-personalization)
