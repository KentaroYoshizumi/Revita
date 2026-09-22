import { createBrowserClient } from "@supabase/ssr";

// A single browser-side Supabase client, used for email/password
// sign-up/sign-in and to read the current session's access token
// before calling the Go backend.
export function createClient() {
  return createBrowserClient(
    process.env.NEXT_PUBLIC_SUPABASE_URL!,
    process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY!
  );
}
