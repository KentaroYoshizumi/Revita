-- Revita evaluation history: every property a user has evaluated,
-- with its full market data / finance figures / Jev verdict / report
-- snapshot, so it can be listed and compared later without paying for
-- another Jev/LLM call (see internal/compare).

create extension if not exists pgcrypto;

create table if not exists public.evaluations (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users (id) on delete cascade,
  address text not null,
  purchase_price double precision not null,
  monthly_rent double precision not null,
  size_sqm double precision not null,
  capacity integer not null,
  business_type text not null default 'minpaku',
  market_data jsonb not null,
  finance_result jsonb not null,
  evaluation jsonb not null,
  report_markdown text not null,
  created_at timestamptz not null default now()
);

create index if not exists evaluations_user_id_created_at_idx
  on public.evaluations (user_id, created_at desc);

alter table public.evaluations enable row level security;

create policy "evaluations_select_own" on public.evaluations
  for select using (auth.uid() = user_id);
