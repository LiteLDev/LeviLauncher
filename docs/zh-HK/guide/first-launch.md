# 首次啟動

第一次開啟 LeviLauncher 時，應用程式會優先確認 Windows 與 Minecraft 環境是否滿足託管安裝要求。

## 你可能會看到什麼

首次啟動時，LeviLauncher 可能會：

- 檢查 Gaming Services 是否可用
- 檢查 GameInput 是否可用
- 驗證 Minecraft Bedrock (GDK) 環境是否已準備就緒
- 在缺失關鍵組件時顯示引導或提示

## 常見首次啟動流程

1. 開啟 LeviLauncher。
2. 閱讀啟動器給出的引導資訊。
3. 安裝或修復缺失的前置組件。
4. 返回啟動器重新檢查狀態。
5. 滿足條件後進入 **Download** 頁面。

## 如果提示缺少組件

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
- 在第一次成功純淨啟動之前，不要急於匯入大量 Mods

## 首次啟動之後建議繼續閱覽

- [版本管理](./version-management)
- [內容管理](./content-management)
- [設定與個人化](./settings-personalization)