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

export interface EvaluateRequest {
  address: string;
  purchase_price: number;
  monthly_rent: number;
  size_sqm: number;
  capacity: number;
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
}

export interface Evaluation {
  Verdict: "Go" | "Conditional" | "NoGo";
  Confidence: number;
  Reasons: Record<string, number>;
}

export interface EvaluateResponse {
  market: MarketData;
  finance: FinanceResult;
  evaluation: Evaluation;
  report_markdown: string;
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
