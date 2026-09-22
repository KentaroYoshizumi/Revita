import Link from "next/link";

export default function BillingSuccessPage() {
  return (
    <main className="mx-auto max-w-sm p-8">
      <h1 className="mb-4 text-2xl font-bold">登録が完了しました</h1>
      <p className="text-sm">
        ご登録ありがとうございます。反映まで数秒かかる場合があります。
      </p>
      <p className="mt-4 text-sm">
        <Link href="/dashboard" className="underline">
          ダッシュボードへ戻る
        </Link>
      </p>
    </main>
  );
}
