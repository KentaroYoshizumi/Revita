import { createClient } from "./supabaseClient";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL!;

export class ApiError extends Error {
  status: number;
  body: unknown;
  constructor(status: number, body: unknown) {
    super(`API error ${status}`);
    this.status = status;
    this.body = body;
  }
}

async function authorizedFetch(path: string, options: RequestInit = {}) {
  const supabase = createClient();
  const { data } = await supabase.auth.getSession();
  const token = data.session?.access_token;
  if (!token) {
    throw new Error("not signed in");
  }

  return fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      ...(options.headers ?? {}),
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
  });
}

export type BusinessType = "minpaku" | "ryokan";

export interface EvaluateRequest {
  address: string;
  purchase_price: number;
  monthly_rent: number;
  size_sqm: number;
  capacity: number;
  business_type: BusinessType;
}

export interface MarketData {
  address: string;
  average_daily_rate: number;
  occupancy_rate: number;
  rev_par: number;
  competitor_count: number;
  currency: string;
}

export interface FinanceResult {
  MonthlyRevenue: number;
  MonthlyProfit: number;
  AnnualProfit: number;
  ROIPercent: number;
  GrossYieldPercent: number;
  NetYieldPercent: number;
  BEPOccupancyRate: number;
  EffectiveOccupancyRate: number;
  LegalCapApplied: boolean;
  LegallyAchievable: boolean;
}

export interface Evaluation {
  Verdict: "Go" | "Conditional" | "NoGo";
  Confidence: number;
  Reasons: Record<string, number>;
}

export interface EvaluateResponse {
  id: string;
  market: MarketData;
  finance: FinanceResult;
  evaluation: Evaluation;
  report_markdown: string;
  phase1_skipped: boolean;
  phase2_skipped: boolean;
  remaining_quota: number;
}

export async function evaluateProperty(
  req: EvaluateRequest
): Promise<EvaluateResponse> {
  const res = await authorizedFetch("/api/evaluate", {
    method: "POST",
    body: JSON.stringify(req),
  });
  const body = await res.json();
  if (!res.ok) {
    throw new ApiError(res.status, body);
  }
  return body as EvaluateResponse;
}

export interface MeResponse {
  user_id: string;
  email: string;
  subscription_status: string;
  subscription_active: boolean;
}

export async function getMe(): Promise<MeResponse> {
  const res = await authorizedFetch("/api/me");
  const body = await res.json();
  if (!res.ok) {
    throw new ApiError(res.status, body);
  }
  return body as MeResponse;
}

export async function startCheckout(): Promise<string> {
  const res = await authorizedFetch("/api/billing/checkout", {
    method: "POST",
  });
  const body = await res.json();
  if (!res.ok) {
    throw new ApiError(res.status, body);
  }
  return body.url as string;
}

export interface EvaluationSummary {
  id: string;
  address: string;
  verdict: "Go" | "Conditional" | "NoGo";
  net_yield_percent: number;
  roi_percent: number;
  created_at: string;
}

export async function listEvaluations(): Promise<EvaluationSummary[]> {
  const res = await authorizedFetch("/api/evaluations");
  const body = await res.json();
  if (!res.ok) {
    throw new ApiError(res.status, body);
  }
  return body as EvaluationSummary[];
}

export interface RankingEntry {
  id: string;
  address: string;
  value: number;
}

export interface CompareResult {
  by_net_yield: RankingEntry[];
  by_bep_margin: RankingEntry[];
  recommended_id: string;
  narrative: string;
}

export async function compareEvaluations(
  ids: string[]
): Promise<CompareResult> {
  const res = await authorizedFetch("/api/evaluations/compare", {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
  const body = await res.json();
  if (!res.ok) {
    throw new ApiError(res.status, body);
  }
  return body as CompareResult;
}
