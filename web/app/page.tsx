"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { createClient } from "@/lib/supabaseClient";

export default function Home() {
  const router = useRouter();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    const supabase = createClient();
    supabase.auth.getSession().then(({ data }) => {
      if (data.session) {
        router.push("/dashboard");
        return;
      }
      setChecking(false);
    });
  }, [router]);

  if (checking) {
    return null;
  }

  return (
    <main className="mx-auto flex max-w-lg flex-col items-center gap-6 p-16 text-center">
      <h1 className="text-3xl font-bold">Revita（レビタ）</h1>
      <p className="text-sm text-gray-600">
        短期賃貸（Airbnb運用）の投資収益性を、Go言語のコアロジック・Jevの一次判定・LLMの詳細レポートで診断します。
      </p>
      <div className="flex gap-4">
        <Link href="/login" className="rounded bg-black px-4 py-2 text-white">
          ログイン
        </Link>
        <Link href="/signup" className="rounded border px-4 py-2">
          新規登録
        </Link>
      </div>
    </main>
  );
}
