# 初回起動

LeviLauncher を初めて開いた際、アプリケーションは Windows および Minecraft 環境がホストインストールの要件を満たしているか確認を優先します。

## 何が表示されるか

初回起動時、LeviLauncher は以下の確認を行う場合があります：

- Gaming Services が利用可能か確認
- GameInput が利用可能か確認
- Minecraft Bedrock (GDK) 環境が準備できているか検証
- 重要なコンポーネントが不足している場合のガイダンスやプロンプトの表示

## 一般的な初回起動フロー

1. LeviLauncher を開く。
2. ランチャーのガイダンス情報を読む。
3. 不足している前提コンポーネントをインストールまたは修復する。
4. ランチャーに戻り、ステータスを再確認する。
5. 条件を満たすと **Download** ページへ進む。

## コンポーネント不足の提示時

### Gaming Services

Gaming Services が不足しているか破損している場合、LeviLauncher は Microsoft Store でのインストールまたは修復を促す場合があります。

### GameInput

GameInput が不足している場合は、プロンプトに従って必要な redistributable をインストールしてください。

### Minecraft Bedrock が検出されない

以下を確認してください：

- 現在の Microsoft アカウントにゲームのライセンスがあるか
- 現在の PC で Microsoft Store からゲームがインストールされているか
- 公式ライセンスフローをバイパスしてランチャーを使用しようとしていないか

## 新規ユーザー向けのおすすめ初期設定

- まず隔離された正式版（Release）バージョンを使用する
- ダウンロードとコンテンツのパスを書き込み可能なディスク場所に置く
- 最初に正常なクリーン起動ができるまで、大量の Mods をインポートしようとしない

## 初回起動後に読むことを推奨するページ

- [バージョン管理](./version-management)
- [コンテンツ管理](./content-management)
- [設定と個人化](./settings-personalization)