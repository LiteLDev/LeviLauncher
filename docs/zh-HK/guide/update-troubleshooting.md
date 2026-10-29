# 更新與故障排查

當 LeviLauncher 的安裝、更新或啟動行為不符合預期時，請優先參考本頁。

## 常見問題 <Badge platform="windows" />

### Gaming Services 缺失或損壞

- 使用啟動器提供的引導進行安裝或修復。
- 必要時返回 Microsoft Store 檢查 Minecraft Bedrock 的安裝狀態。

### GameInput 缺失

- 按 LeviLauncher 提示安裝所需的 Redistributable 組件。
- 安裝完成後，如有需要請重新啟動啟動器。

### 安裝路徑不可寫入

- 在 **設定 → 基礎設定** 中將託管內容路徑更改為可寫入的位置。
- 如果啟動器在安裝或自我更新時要求提升權限，僅在你確認可信時允許執行。

### LeviLauncher 一閃即退或從不顯示視窗

- 在 Windows 上，LeviLauncher 現在會以更保守的 WebView2 軟件渲染模式啟動，以減少部分顯示卡與驅動程式組合下的啟動崩潰。
- 如果介面出現前啟動器仍然退出，請先檢查 `%APPDATA%\levilauncher.exe\logs\startup.log`。
- 啟動失敗後可先重試一次；如果仍然出現，提交 GitHub Issue 時請附上 `startup.log`。

### 某個版本無法啟動

- 先測試該版本在無模組情況下能否啟動
- 檢查所需的 Windows 組件是否仍然存在
- 確認該版本是否已經完整安裝
- 嘗試建立一個乾淨的隔離測試環境以復現問題

### 下載很慢或反覆失敗

- 切換鏡像
- 稍後重試
- 如條件允許，改用本地包來源

## Android 上的常見問題 <Badge platform="android" />

### 找不到 Minecraft 或版本不受支援

從 Google Play 安裝或更新官方 Minecraft（1.21.80 或更高版本），然後重新打開 LeviLauncher。

### 匯入版本無法啟動

匯入版本通常需要版本隔離。請在實例設定中啟用後重試。

### 啟動失敗

從失敗頁面打開啟動日誌，先查看報告的原因，再調整設定。

### 資料遷移

啟動器可能需要先遷移舊的資料目錄。在遷移成功、並確認世界、資源包和設定都可用之前，不要刪除舊目錄。

## 更安全的恢復步驟

1. 先備份重要世界。
2. 移除最近新增的模組或資源包。
3. 用一個乾淨的隔離版本作對照測試。
4. 必要時重新安裝受影響的版本。

## 關於自我更新 <Badge platform="windows" />

LeviLauncher 可以檢查、下載並安裝應用程式更新。若安裝目錄不可寫入，在某些環境下可能需要管理員權限。

## 什麼時候應該提交 Issue
當滿足以下條件時，建議到 GitHub 提交問題：

- 你能穩定復現問題
- 做過基礎恢復步驟後問題仍然存在
- 問題看起來更像是 LeviLauncher 自身所致，而非一般的 Windows 環境問題

提交時請盡量附上：

- Windows 版本
- LeviLauncher 版本
- 你的具體操作步驟
- 截圖或日誌（如果有）

問題反饋入口：[GitHub Issues](https://github.com/LiteLDev/LeviLauncher/issues)

Android 版的問題請附上 Android 版本、裝置型號和啟動日誌，並提交到 [LiteLDev/LeviLaunchroid](https://github.com/LiteLDev/LeviLaunchroid/issues)。
