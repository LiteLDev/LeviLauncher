# 模組與整合

LeviLauncher 支援超出「基礎啟動器」範圍的模組相關工作流程。

## 當前可用功能

=== "Windows"

    - 匯入模組包（`.mcmod`）
    - 仍相容 `.zip` 包及單獨的 `.dll` 模組：在檔案選擇框中選擇 **所有檔案 (\*.\*)** 即可
    - 啟用或停用已安裝的模組
    - 刪除不再需要的模組
    - 使用 CurseForge 相關瀏覽與獲取流程
    - 使用相容 [lip](https://github.com/LiteLDev/lip) 的內置套件管理器安裝 LIP 套件
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

LeviLauncher 內置了相容 [lip](https://github.com/LiteLDev/lip) 的套件管理器，毋須另外安裝 lip 或 .NET 執行階段。它遵循 lip 0.38：每個版本的套件記錄在 `.lip/workspace.yaml` 中，套件從其 `lip.yaml` 讀取（只有 `tooth.json` 的舊套件仍可使用）。用舊版啟動器安裝過 LeviLamina 或 LIP 套件的版本（`tooth_lock.json`）會在第一次套件操作時自動遷移。開啟 **使用鏡像來源** 時，從 GitHub 下載套件會經過[設定](./settings-personalization)中的 **GitHub 代理**；否則直接連線 GitHub（如已設定系統代理，則經過系統代理）。之前儲存過的 GitHub 代理，或 lip 的 `config.yaml`（位於 `%APPDATA%\lip`，例如透過 `lip config set github_proxy <URL>` 設定）中的 `github_proxy`，在兩種情況下都會生效。**清理快取** 可刪除已下載的套件檔案。

這類集成更適合希望使用包驅動或加載器驅動工作流程的進階用戶。

最佳實踐：

- 先從穩定的正式版（Release）版本開始
- 不要一次變更多個高風險因素
- 記錄下「最後一次成功運作」的版本組合

要為用戶端版本加入 LeviLamina，請參閱 LeviLamina 文檔中的[在Windows上安裝用戶端](https://docs.levimc.org/LeviLamina/zh-Hant/user_guides/install_on_client/)。

## 原生模組 <Badge platform="android" />

原生模組會在遊戲程序中執行原生程式碼。只安裝來自可信來源的模組，並先在隔離版本中測試。

如果要自己開發，請參閱 LeviLamina 文檔中的 [Android 原生模組](https://docs.levimc.org/LeviLamina/zh-Hant/developer_guides/android/)。

## 如果加了模組後出錯

- 先停用最新加入的模組
- 與上一個可正常運作的狀態進行對比
- 進一步測試前先轉移或備份重要世界
- 然後繼續閱覽[更新與故障排查](./update-troubleshooting)
