"use client";

import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { createClient } from "@/lib/supabaseClient";
import {
  ApiError,
  evaluateProperty,
  getMe,
  startCheckout,
  type EvaluateResponse,
  type MeResponse,
} from "@/lib/api";

const VERDICT_LABEL: Record<string, string> = {
  Go: "Go（投資に前向き）",
  Conditional: "Conditional（条件付き）",
  NoGo: "NoGo（見送り推奨）",
};

export default function DashboardPage() {
  const router = useRouter();
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [me, setMe] = useState<MeResponse | null>(null);
  const [meError, setMeError] = useState<string | null>(null);

  const [address, setAddress] = useState("東京都渋谷区神南1-1-1");
  const [price, setPrice] = useState(45000000);
  const [rent, setRent] = useState(150000);
  const [size, setSize] = useState(32.5);
  const [capacity, setCapacity] = useState(4);

  const [result, setResult] = useState<EvaluateResponse | null>(null);
  const [evalError, setEvalError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [checkoutLoading, setCheckoutLoading] = useState(false);

  useEffect(() => {
    const supabase = createClient();
    supabase.auth.getSession().then(({ data }) => {
      if (!data.session) {
        router.push("/login");
        return;
      }
      setCheckingAuth(false);
      refreshMe();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function refreshMe() {
    try {
      setMe(await getMe());
      setMeError(null);
    } catch (err) {
      setMeError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleSubscribe() {
    setCheckoutLoading(true);
    try {
      const url = await startCheckout();
      window.location.href = url;
    } catch (err) {
      setEvalError(err instanceof Error ? err.message : String(err));
      setCheckoutLoading(false);
    }
  }

  async function handleSignOut() {
    const supabase = createClient();
    await supabase.auth.signOut();
    router.push("/login");
  }

  async function handleEvaluate(e: FormEvent) {
    e.preventDefault();
    setLoading(true);
    setEvalError(null);
    setResult(null);

    try {
      const res = await evaluateProperty({
        address,
        purchase_price: price,
        monthly_rent: rent,
        size_sqm: size,
        capacity,
      });
      setResult(res);
      refreshMe();
    } catch (err) {
      if (err instanceof ApiError && err.status === 402) {
        setEvalError(
          "有効なサブスクリプションがありません。下記から登録してください。"
        );
      } else if (err instanceof ApiError && err.status === 429) {
        setEvalError("今月の実行回数の上限に達しました。来月またお試しください。");
      } else {
        setEvalError(err instanceof Error ? err.message : String(err));
      }
    } finally {
      setLoading(false);
    }
  }

  if (checkingAuth) {
    return <main className="p-8">読み込み中...</main>;
  }

  return (
    <main className="mx-auto max-w-2xl p-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">Revita: 物件収益性チェック</h1>
        <button onClick={handleSignOut} className="text-sm underline">
          ログアウト
        </button>
      </div>

      <section className="mb-6 rounded border p-4">
        {meError && <p className="text-sm text-red-600">{meError}</p>}
        {me && (
          <div className="flex items-center justify-between text-sm">
            <span>
              {me.email} / サブスクリプション:{" "}
              <strong>
                {me.subscription_active ? "有効" : "未登録・停止中"}
              </strong>
            </span>
            {!me.subscription_active && (
              <button
                onClick={handleSubscribe}
                disabled={checkoutLoading}
                className="rounded bg-black px-3 py-1.5 text-white disabled:opacity-50"
              >
                {checkoutLoading ? "処理中..." : "月額300円で登録する"}
              </button>
            )}
          </div>
        )}
      </section>

      <form onSubmit={handleEvaluate} className="mb-8 flex flex-col gap-4">
        <label className="flex flex-col gap-1 text-sm">
          住所
          <input
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            required
            className="rounded border px-3 py-2"
          />
        </label>
        <div className="grid grid-cols-2 gap-4">
          <label className="flex flex-col gap-1 text-sm">
            購入価格（円）
            <input
              type="number"
              value={price}
              onChange={(e) => setPrice(Number(e.target.value))}
              required
              className="rounded border px-3 py-2"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            月額費用（円）
            <input
              type="number"
              value={rent}
              onChange={(e) => setRent(Number(e.target.value))}
              required
              className="rounded border px-3 py-2"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            広さ（㎡）
            <input
              type="number"
              step="0.1"
              value={size}
              onChange={(e) => setSize(Number(e.target.value))}
              required
              className="rounded border px-3 py-2"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            定員（人）
            <input
              type="number"
              value={capacity}
              onChange={(e) => setCapacity(Number(e.target.value))}
              required
              className="rounded border px-3 py-2"
            />
          </label>
        </div>
        {evalError && <p className="text-sm text-red-600">{evalError}</p>}
        <button
          type="submit"
          disabled={loading}
          className="rounded bg-black px-3 py-2 text-white disabled:opacity-50"
        >
          {loading ? "評価中..." : "評価する"}
        </button>
      </form>

      {result && (
        <section className="flex flex-col gap-4">
          <div className="rounded border p-4">
            <h2 className="mb-2 font-bold">市場データ</h2>
            <p className="text-sm">
              ADR: {result.market.average_daily_rate.toFixed(0)}円 / 稼働率:{" "}
              {(result.market.occupancy_rate * 100).toFixed(1)}% / 競合物件数:{" "}
              {result.market.competitor_count}件
            </p>
          </div>

          <div className="rounded border p-4">
            <h2 className="mb-2 font-bold">想定収支</h2>
            <ul className="text-sm">
              <li>想定月間損益: {result.finance.MonthlyProfit.toFixed(0)}円</li>
              <li>年間ROI: {result.finance.ROIPercent.toFixed(2)}%</li>
              <li>
                表面利回り: {result.finance.GrossYieldPercent.toFixed(2)}%
              </li>
              <li>実質利回り: {result.finance.NetYieldPercent.toFixed(2)}%</li>
              <li>
                損益分岐稼働率:{" "}
                {(result.finance.BEPOccupancyRate * 100).toFixed(1)}%
              </li>
            </ul>
          </div>

          <div className="rounded border p-4">
            <h2 className="mb-2 font-bold">
              Phase 1 判定: {VERDICT_LABEL[result.evaluation.Verdict]}（確度{" "}
              {(result.evaluation.Confidence * 100).toFixed(0)}%）
            </h2>
            {result.phase2_skipped && (
              <p className="mb-2 text-sm text-gray-600">
                NoGo判定のため、Phase 2（詳細レポート生成）はスキップされました。
              </p>
            )}
            <pre className="whitespace-pre-wrap text-sm">
              {result.report_markdown}
            </pre>
          </div>

          <p className="text-xs text-gray-500">
            今月の残り実行可能回数: {result.remaining_quota}回
          </p>
        </section>
      )}
    </main>
  );
}
