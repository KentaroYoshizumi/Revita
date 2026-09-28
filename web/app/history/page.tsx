"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { createClient } from "@/lib/supabaseClient";
import {
  compareEvaluations,
  listEvaluations,
  type CompareResult,
  type EvaluationSummary,
} from "@/lib/api";

const VERDICT_LABEL: Record<string, string> = {
  Go: "Go",
  Conditional: "Conditional",
  NoGo: "NoGo",
};

export default function HistoryPage() {
  const router = useRouter();
  const [checkingAuth, setCheckingAuth] = useState(true);
  const [evaluations, setEvaluations] = useState<EvaluationSummary[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [listError, setListError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const [compareResult, setCompareResult] = useState<CompareResult | null>(
    null
  );
  const [compareError, setCompareError] = useState<string | null>(null);
  const [comparing, setComparing] = useState(false);

  async function refreshList() {
    setLoading(true);
    try {
      setEvaluations(await listEvaluations());
      setListError(null);
    } catch (err) {
      setListError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    const supabase = createClient();
    supabase.auth.getSession().then(({ data }) => {
      if (!data.session) {
        router.push("/login");
        return;
      }
      setCheckingAuth(false);
      refreshList();
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function toggleSelected(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  async function handleCompare() {
    setComparing(true);
    setCompareError(null);
    setCompareResult(null);
    try {
      setCompareResult(await compareEvaluations(Array.from(selected)));
    } catch (err) {
      setCompareError(err instanceof Error ? err.message : String(err));
    } finally {
      setComparing(false);
    }
  }

  if (checkingAuth) {
    return <main className="p-8">読み込み中...</main>;
  }

  return (
    <main className="mx-auto max-w-3xl p-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">評価履歴・比較</h1>
        <Link href="/dashboard" className="text-sm underline">
          ダッシュボードへ戻る
        </Link>
      </div>

      {listError && <p className="mb-4 text-sm text-red-600">{listError}</p>}

      {loading ? (
        <p className="text-sm text-gray-500">読み込み中...</p>
      ) : evaluations.length === 0 ? (
        <p className="text-sm text-gray-500">
          まだ評価履歴がありません。ダッシュボードから物件を評価すると、ここに記録されます。
        </p>
      ) : (
        <>
          <table className="mb-4 w-full text-sm">
            <thead>
              <tr className="border-b text-left">
                <th className="py-2"></th>
                <th className="py-2">住所</th>
                <th className="py-2">判定</th>
                <th className="py-2">実質利回り</th>
                <th className="py-2">ROI</th>
                <th className="py-2">評価日時</th>
              </tr>
            </thead>
            <tbody>
              {evaluations.map((ev) => (
                <tr key={ev.id} className="border-b">
                  <td className="py-2">
                    <input
                      type="checkbox"
                      checked={selected.has(ev.id)}
                      onChange={() => toggleSelected(ev.id)}
                    />
                  </td>
                  <td className="py-2">{ev.address}</td>
                  <td className="py-2">{VERDICT_LABEL[ev.verdict]}</td>
                  <td className="py-2">{ev.net_yield_percent.toFixed(2)}%</td>
                  <td className="py-2">{ev.roi_percent.toFixed(2)}%</td>
                  <td className="py-2 text-gray-500">
                    {new Date(ev.created_at).toLocaleString("ja-JP")}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <button
            onClick={handleCompare}
            disabled={selected.size < 2 || comparing}
            className="rounded bg-black px-3 py-2 text-sm text-white disabled:opacity-50"
          >
            {comparing
              ? "比較中..."
              : `選択した${selected.size}件を比較する（2件以上選択してください）`}
          </button>
        </>
      )}

      {compareError && (
        <p className="mt-4 text-sm text-red-600">{compareError}</p>
      )}

      {compareResult && (
        <section className="mt-6 flex flex-col gap-4">
          <div className="rounded border p-4">
            <h2 className="mb-2 font-bold">比較結果</h2>
            <p className="text-sm">{compareResult.narrative}</p>
          </div>

          <div className="rounded border p-4">
            <h3 className="mb-2 font-bold">実質利回り ランキング</h3>
            <ol className="list-inside list-decimal text-sm">
              {compareResult.by_net_yield.map((entry) => (
                <li key={entry.id}>
                  {entry.address}: {entry.value.toFixed(2)}%
                </li>
              ))}
            </ol>
          </div>

          <div className="rounded border p-4">
            <h3 className="mb-2 font-bold">損益分岐マージン ランキング</h3>
            <ol className="list-inside list-decimal text-sm">
              {compareResult.by_bep_margin.map((entry) => (
                <li key={entry.id}>
                  {entry.address}: {entry.value.toFixed(1)}ポイント
                </li>
              ))}
            </ol>
          </div>
        </section>
      )}
    </main>
  );
}
