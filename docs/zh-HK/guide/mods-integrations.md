# 模組與整合

LeviLauncher 支援超出「基礎啟動器」範圍的模組相關工作流程。

## 當前可用功能

=== "Windows"

    - 匯入 `.zip` 模組
    - 匯入 `.dll` 模組
    - 啟用或停用已安裝的模組
    - 刪除不再需要的模組
    - 使用 CurseForge 相關瀏覽與獲取流程
    - 使用套件安裝器 [lip](https://github.com/LiteLDev/lip) 安裝套件
    - 配合 LeviLamina 等加載器的工作流程

    ::: warning 這是進階區域
    模組、加載器與套件安裝整合的相容性變化往往較快，尤其是在 Minecraft 預覽版（Preview）上。請將此處視為進階功能，並逐步進行測試。
    :::

=== "Android"

    - 向遊戲載入原生模組（`.so` 程式庫）
    - 在隔離版本中維護帶模組的環境

    想開發原生模組？請參閱 LeviLamina 文檔中的 [Android 原生模組](https://docs.levimc.org/LeviLamina/zh-Hant/developer_guides/android/)。

## 建議的模組工作流程

1. 從一個乾淨的隔離版本開始。
2. 先確認該版本在無模組情況下可以正常啟動。
3. 一次只增加一個模組或一小組改動。
4. 每次改動後都重新測試啟動。
5. 重要世界始終保留備份路徑。

## CurseForge <Badge platform="windows" />

啟動器提供 CurseForge 相關瀏覽與包流程，協助你在應用程式內更輕鬆地尋找合適資源。

適合以下需求：

- 需要更具引導式的搜尋體驗
- 想更快查看某個項目的相容檔案
- 不希望所有搜尋都在啟動器外完成

## lip 與 LeviLamina <Badge platform="windows" />

這類集成更適合希望使用包驅動或加載器驅動工作流程的進階用戶。

最佳實踐：

- 先從穩定的正式版（Release）版本開始
- 不要一次變更多個高風險因素
- 記錄下「最後一次成功運作」的版本組合

## 原生模組 <Badge platform="android" />

原生模組會在遊戲程序中執行原生程式碼。只安裝來自可信來源的模組，並先在隔離版本中測試。

如果要自己開發，請參閱 LeviLamina 文檔中的 [Android 原生模組](https://docs.levimc.org/LeviLamina/zh-Hant/developer_guides/android/)。

## 如果加了模組後出錯

- 先停用最新加入的模組
- 與上一個可正常運作的狀態進行對比
- 進一步測試前先轉移或備份重要世界
- 然後繼續閱覽[更新與故障排查](./update-troubleshooting)
