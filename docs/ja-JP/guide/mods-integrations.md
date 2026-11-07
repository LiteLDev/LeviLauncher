# Mod と統合

LeviLauncher は、「基本ランチャー」の範囲を超えた Mod 関連のワークフローをサポートしています。

## 現在利用可能な機能

=== "Windows"

    - Mod パッケージ（`.mcmod`）のインポート
    - `.zip` パッケージや単体の `.dll` Mod も引き続き使用可能：ファイル選択ダイアログで **すべてのファイル (\*.\*)** を選択
    - インストール済み Mod の有効化または無効化
    - 不要になった Mod の削除
    - CurseForge 関連の閲覧および取得フローの使用
    - [lip](https://github.com/LiteLDev/lip) 互換の内蔵パッケージマネージャーで LIP パッケージをインストール
    - LeviLamina などのローダーワークフローとの連携

    ::: warning これは高度な領域です
    Mod、ローダー、パッケージ統合の互換性に関する変更は、特に Minecraft プレビュー版（Preview）において、より頻繁に起こる傾向があります。ここは高度な機能として扱い、段階的にテストしてください。
    :::

=== "Android"

    - ネイティブ Mod（`.so` ライブラリ）をゲームに読み込む
    - Mod 入りの環境を分離したバージョンで保つ

    ネイティブ Mod を作りたい場合は、LeviLamina ドキュメントの [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/)（英語）を参照してください。

## 推奨される Mod ワークフロー

1. クリーンな隔離バージョンから始める。
2. Mod なしの状態でそのバージョンが正常に起動することを確認する。
3. Mod または変更を一度に 1 つ、または少数のグループずつ追加する。
4. 変更のたびに起動を再テストする。
5. 重要なワールドは常にバックアップの道筋を確保しておく。

## CurseForge <Badge platform="windows" />

ランチャーは CurseForge 関連の閲覧およびパッケージフローを提供しており、アプリ内で適切なリソースをより簡単に見つけることができます。

以下のニーズに適しています：

- よりガイド付きの検索体験が必要な場合
- 特定プロジェクトの互換ファイルをより迅速に確認したい場合
- 検索のすべてをランチャー外で行いたくない場合

## lip と LeviLamina <Badge platform="windows" />

LeviLauncher には [lip](https://github.com/LiteLDev/lip) 互換のパッケージマネージャーが内蔵されているため、lip や .NET ランタイムを別途インストールする必要はありません。lip 0.38 に準拠しており、各バージョンはパッケージを `.lip/workspace.yaml` に記録し、パッケージは `lip.yaml` から読み込まれます（`tooth.json` しかない古いパッケージも引き続き使えます）。以前のランチャーで LeviLamina や LIP パッケージをインストールしたバージョン（`tooth_lock.json`）は、最初のパッケージ操作時に自動的に移行されます。**ミラーソースを使用** がオンのときは、GitHub からのパッケージのダウンロードに[設定](./settings-personalization)の **GitHub プロキシ** が使われます。オフのときは GitHub に直接接続します（システムプロキシが設定されていればそれを経由します）。以前に保存した GitHub プロキシや、lip の `config.yaml`（`%APPDATA%\lip` 内。例: `lip config set github_proxy <URL>`）で設定した `github_proxy` はどちらの場合にも適用されます。**キャッシュを削除** でダウンロード済みのパッケージファイルを削除できます。

これらの統合は、パッケージ駆動またはローダー駆動のワークフローを使用したい上級ユーザーに最適です。

ベストプラクティス：

- まず安定した正式版（Release）バージョンから始める
- 一度に複数の高リスク要素を変更しない
- 「最後に正常に動作した」バージョンの組み合わせを記録しておく

クライアントのバージョンに LeviLamina を追加するには、LeviLamina ドキュメントの [Install client on Windows](https://docs.levimc.org/LeviLamina/user_guides/install_on_client/)（英語）を参照してください。

## ネイティブ Mod <Badge platform="android" />

ネイティブ Mod はゲームのプロセス内でネイティブコードを実行します。信頼できる入手元の Mod だけをインストールし、まず分離したバージョンで試してください。

自分で作る場合は、LeviLamina ドキュメントの [Android native mods](https://docs.levimc.org/LeviLamina/developer_guides/android/)（英語）に従ってください。

## Mod 追加後にエラーが発生した場合

- 最新に追加した Mod をまず無効にする
- 直前に正常に動作していた状態と比較する
- さらなるテストを行う前に、重要なワールドを移動またはバックアップする
- その後、[更新とトラブルシューティング](./update-troubleshooting) を参照する
