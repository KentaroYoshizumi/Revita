import Link from "next/link";

export default function BillingCancelPage() {
  return (
    <main className="mx-auto max-w-sm p-8">
      <h1 className="mb-4 text-2xl font-bold">登録がキャンセルされました</h1>
      <p className="text-sm">決済は行われていません。</p>
      <p className="mt-4 text-sm">
        <Link href="/dashboard" className="underline">
          ダッシュボードへ戻る
        </Link>
      </p>
    </main>
  );
}
