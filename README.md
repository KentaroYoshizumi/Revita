# Revita（レビタ）

不動産投資（短期賃貸／Airbnb運用）の収益性をチェックするツール。
現在はフロントエンドを後回しにし、Go言語によるCLIのコアロジックのみを実装したプロトタイプです。

## アーキテクチャ: 2段階（2-Phase）評価パイプライン

1. **数値計算（Go）**: ROI・BEP（損益分岐稼働率）・表面/実質利回りをGoで正確に事前計算する（`internal/finance`）
2. **Phase 1（Jev）**: Go計算結果と市場データを判断特化型AIモデル「Jev」(TypeSafe AI)に渡し、高速・低コストで構造化判定（`"Go" | "Conditional" | "NoGo"`）と確度スコアを得る（`internal/jev`）
3. **Phase 2（LLM）**: Jevの判定結果を受け取り、Claude/OpenAI APIでユーザー向けの詳細なMarkdownレポートを生成する（`internal/llm`）
4. **コスト最適化**: Phase 1が`NoGo`と判定した場合は、Phase 2（LLMによる長文生成）をスキップし、代わりにJevの結果のみによる簡易サマリーを返す（`internal/pipeline`）

## 機能

1. コマンドラインフラグで物件情報（住所・購入価格・月額費用・広さ・定員）を受け取る
2. 短期賃貸の市場データ（稼働率・平均日次単価・競合物件数）を取得する
   - **稼働率**: 観光庁「宿泊旅行統計調査」（e-Stat経由、無料・全国対応）
   - **競合物件数**: 観光庁の住宅宿泊事業（民泊）届出データ（無料、ローカルCSVから読込）
   - **ADR（平均日次単価）**: 無料の政府データに相当するものがないため、当面はモック値
   - `ESTAT_APP_ID`未設定時、または取得に失敗した場合は、全項目をAirDNA形状のモックデータにフォールバックする
3. 物件コストと市場データから想定収支（月間収入・月間損益・年間損益・年間ROI・表面/実質利回り・損益分岐稼働率）を計算する
4. Phase 1: 計算結果と市場データをJevに渡し、Go/Conditional/NoGoの一次判定と確度スコアを得る
   - `TYPESAFE_API_KEY`未設定の場合は、ROI・稼働率マージンに基づくルールベースのモック判定にフォールバックする
5. Phase 2: 一次判定がNoGoでなければ、LLM（Claude APIまたはOpenAI API）で詳細なMarkdownレポートを生成する
   - NoGo判定、またはLLM APIキー未設定の場合は、Jevの結果のみによる簡易サマリーにフォールバックする（APIコスト削減）

## 使い方

```bash
go run ./cmd/revita \
  --address="東京都渋谷区神南1-1-1" \
  --price=45000000 \
  --rent=150000 \
  --size=32.5 \
  --capacity=4
```

フラグを省略するとサンプル物件情報で実行されます。

### Phase 1（Jev）を実データ化する

```bash
export TYPESAFE_API_KEY=your-typesafe-api-key
```

未設定の場合は、年間ROI(目安8%以上)と損益分岐稼働率に対する稼働率マージン(目安10ポイント以上)を基準にしたルールベースのモック判定（`internal/jev.MockClient`）が使われます。

### 市場データを実データ化する（e-Stat / 民泊届出データ）

以下の環境変数を両方設定すると、稼働率を観光庁の統計データ（e-Stat経由）から取得します。

```bash
export ESTAT_APP_ID=your-estat-app-id
export ESTAT_STATS_DATA_ID=your-stats-data-id
```

- `ESTAT_APP_ID`: [e-Stat](https://www.e-stat.go.jp/mypage/)に無料登録すると発行される、アプリケーションID
- `ESTAT_STATS_DATA_ID`: 「宿泊旅行統計調査」の「客室稼働率（都道府県別）」の統計表IDです。e-Statのサイト内検索から該当の統計表を探し、その「API」タブに表示されるIDを設定してください。国が確定値を出すタイミングで統計表IDが変わることがあるため、定期的な見直しが必要です。

いずれか一方でも未設定・不正、またはAPI呼び出しに失敗した場合は、稼働率を含めた全データがモック値にフォールバックします。

競合物件数は、観光庁が公表する都道府県別の民泊届出住宅数をCSVで用意すると実データに置き換わります。

```bash
go run ./cmd/revita --minpaku-csv=data/minpaku_todokede.csv ...
```

`data/minpaku_todokede.csv`はヘッダーのみのテンプレートです。[住宅宿泊事業法の施行状況（観光庁）](https://www.mlit.go.jp/kankocho/minpaku/business/host/construction_situation.html)で公表されている都道府県別の届出住宅数を、以下の形式で追記してください。

```csv
都道府県,届出住宅数
東京都,12345
福岡県,678
```

該当都道府県のデータがCSVに無い場合は、競合物件数のみ固定値（24件）にフォールバックします。

### Phase 2（LLM詳細レポート）を有効にする

以下のいずれかの環境変数を設定すると、実際のLLM APIによる詳細レポート生成が行われます（Claude APIを優先）。

```bash
export ANTHROPIC_API_KEY=sk-ant-...
# または
export OPENAI_API_KEY=sk-...
```

いずれも未設定の場合、またはPhase 1がNoGoと判定した場合は、Jevの結果のみによる簡易Markdownサマリーを表示します。

## ディレクトリ構成

```
cmd/revita/            CLIエントリポイント（2段階パイプラインの起動）
internal/property/     物件情報の型定義
internal/airdna/       市場データの取得（モック/政府統計データクライアント、共通のClientインターフェース）
internal/estat/        e-Stat（政府統計の総合窓口）APIクライアント（稼働率取得）
internal/minpaku/      民泊届出住宅数CSVの読込
internal/prefecture/   住所から都道府県を判定するユーティリティ
internal/finance/      想定収支・ROI・BEP・表面/実質利回りの計算ロジック（Goによる事前計算）
internal/jev/          Phase 1: Jev（TypeSafe AI）APIクライアントとモック判定
internal/llm/          Phase 2: LLM APIによる詳細Markdownレポート生成
internal/pipeline/     2段階評価パイプラインの実行ロジック（NoGo時のPhase2スキップを含む）
data/                  民泊届出住宅数CSVなどのローカルデータ
```

## 今後の予定

- ADR（平均日次単価）を実データ化する有料APIへの切替（AirDNA、AirROI等）
- 画面（フロントエンド）の実装
