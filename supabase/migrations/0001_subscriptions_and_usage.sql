-- Revita SaaS schema: subscription state (Stripe) and monthly usage
-- counters (rate limiting), keyed to Supabase Auth's auth.users.
--
-- Run this in the Supabase project's SQL editor, or via the Supabase
-- CLI (`supabase db push`), after enabling Auth (email/password) on the
-- project.

create table if not exists public.subscriptions (
  user_id uuid primary key references auth.users (id) on delete cascade,
  stripe_customer_id text,
  stripe_subscription_id text,
  -- Stripe subscription status: "active", "trialing", "past_due",
  -- "canceled", "incomplete", "incomplete_expired", "unpaid", etc.
  -- A user may run evaluations only while status is "active" or
  -- "trialing" (see internal/billing.IsUsable).
  status text not null default 'none',
  current_period_end timestamptz,
  updated_at timestamptz not null default now()
);

create table if not exists public.usage_counters (
  user_id uuid not null references auth.users (id) on delete cascade,
  -- Calendar month the counter applies to, formatted "YYYY-MM" in UTC.
  year_month text not null,
  count integer not null default 0,
  updated_at timestamptz not null default now(),
  primary key (user_id, year_month)
);

alter table public.subscriptions enable row level security;
alter table public.usage_counters enable row level security;

-- The Go backend writes through the service-role key, which bypasses
-- RLS. These policies only cover read access from the Next.js frontend
-- calling Supabase directly with the user's own session.
create policy "subscriptions_select_own" on public.subscriptions
  for select using (auth.uid() = user_id);

create policy "usage_counters_select_own" on public.usage_counters
  for select using (auth.uid() = user_id);
