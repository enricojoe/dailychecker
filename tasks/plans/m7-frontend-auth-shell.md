# M7 — Frontend: Auth & Shell

> Execution plan for Milestone 7. Parent plan: [dailychecker.md](dailychecker.md) §Milestone 7.

## Goal
Login/register pages, token storage, auto-refresh on 401, logout, an app shell with protected routing, and the API client + TanStack Query setup — so a user can register, log in, stay logged in across a page refresh, and log out. (Feature pages — activities/today/history/telegram — are M8; M7 ships the shell with a placeholder landing.)

## Existing scaffold (M0 — reuse, don't rebuild)
- Vite 8 + React 19 + TS, Tailwind v4, shadcn/base-ui (`@/components/ui/button`), `@` alias → `src`.
- `src/main.tsx`: `QueryClientProvider` already set up (throwOnError on queries, retry 1, staleTime 1m).
- `src/api/apiClient.ts`: a `request<T>` fetch wrapper + `apiClient.{get,post,patch,delete}` reading `import.meta.env.VITE_API_BASE_URL`. **Extend this** (auth header + refresh) rather than replacing it. `src/api/index.ts` re-exports `apiClient`.
- `src/App.tsx`: placeholder landing — replace with the router/shell.
- `src/auth/`, `src/routes/`, `src/features/*` exist as empty dirs.
- `react-router-dom@7` is installed (orchestrator added it). TanStack Query v5 installed.
- Env: `VITE_API_BASE_URL` (default `http://localhost:8080/api`).

## Backend API contract (already built & tested, M2)
- `POST /api/auth/register` `{name, phone, password}` → `201 {id, name, phone, created_at, updated_at}` | `409` (phone taken) | `422` (validation). Password min length 8.
- `POST /api/auth/login` `{phone, password}` → `200 {access, refresh}` | `401`.
- `POST /api/auth/refresh` `{refresh}` → `200 {access, refresh}` (**rotated** — old refresh is revoked) | `401`.
- `POST /api/auth/logout` `{refresh}` → `204` | `401`.
- `GET /api/me` (Bearer access) → `200 {id, name, phone, telegram_chat_id?, telegram_linked_at?, ...}` | `401`.
- Error envelope on failures: `{ "error": "message" }`.

## Design decisions
1. **Token storage:** persist BOTH `access` and `refresh` in `localStorage` (so login survives a hard refresh — the DoD). Access is short-lived (~15m); the refresh flow covers its expiry. Document the XSS trade-off (acceptable for v1; revisit in M9). Centralize get/set/clear in one module (`src/auth/tokenStore.ts`).
2. **Auto-refresh on 401:** in the `request` wrapper — attach `Authorization: Bearer <access>`; on a `401`, attempt ONE refresh via `POST /api/auth/refresh {refresh}`, store the rotated pair, and retry the original request once. If refresh fails (or no refresh token), clear tokens and signal logged-out (redirect to `/login`). Guard against concurrent refreshes (single in-flight refresh promise shared by parallel 401s — avoid a refresh storm, important because refresh tokens ROTATE so two parallel refreshes would invalidate each other).
3. **Auth state:** an `AuthProvider` (`src/auth/AuthContext.tsx`) exposing `{ user, isAuthenticated, isLoading, login(phone,password), register(...), logout() }`. Use TanStack Query for `GET /api/me` (enabled when a token exists) to hydrate `user` on load. `login`/`register` are mutations; on success store tokens + invalidate/refetch `me`. `logout` calls the API, clears tokens, resets query cache.
4. **Routing (`src/routes/`):** public `/login`, `/register`; protected everything else via a `<ProtectedRoute>` that redirects to `/login` when unauthenticated (and shows a loading state while `me` resolves). Authenticated users hitting `/login` or `/register` redirect to `/`.
5. **App shell:** header with app name + the logged-in user's name + a Logout button; a nav area (links/placeholders for Today/Activities/History/Telegram — wired in M8); main content outlet. `/` renders the shell with a simple placeholder landing ("Today — coming in M8").

## Tasks
- [ ] `src/auth/tokenStore.ts` — get/set/clear access+refresh in localStorage.
- [ ] Extend `src/api/apiClient.ts` — Bearer injection, single-flight 401→refresh→retry, clear-on-failure hook. Keep the existing `apiClient.{get,post,...}` surface.
- [ ] `src/auth/api.ts` (or fold into a feature) — typed calls: `register`, `login`, `refresh`, `logout`, `me` + DTO types.
- [ ] `src/auth/AuthContext.tsx` + `useAuth()` hook — provider with login/register/logout + `me` query.
- [ ] `src/routes/` — router config; `ProtectedRoute`; public-only redirect.
- [ ] Pages: `src/features/auth/LoginPage.tsx`, `RegisterPage.tsx` — controlled forms, client validation (phone required; password ≥ 8), surface backend `{error}` (e.g. 409 "phone taken", 401 "invalid credentials"), loading/disabled states.
- [ ] App shell component + wire `App.tsx`/`main.tsx` (AuthProvider inside QueryClientProvider, RouterProvider).
- [ ] Use existing shadcn/Tailwind primitives; add small UI pieces (input/label/card) only as needed, matching the existing Button style.

## DoD (parent plan)
User can register, log in, stay logged in across a page refresh, and log out. Plus: `npx tsc --noEmit` clean, `npm run build` clean, `npm run lint` clean.

## Constraints
- Idiomatic modern React (function components, hooks), TS strict, accessible forms (labels, error messaging). Match existing scaffold conventions (`@` alias, Tailwind v4, shadcn/base-ui). Keep components focused; no premature abstraction.
- NEVER store backend secrets in the frontend. Don't build M8 feature pages (activities/today/history beyond a placeholder). Don't touch the backend. Don't commit — leave changes for orchestrator review.
- Don't add heavy deps without need (react-router-dom + TanStack Query already present; plain controlled forms are fine — no form library required).

## Verification (the subagent must run these — all local, no network)
- `npx tsc --noEmit` clean.
- `npm run build` clean.
- `npm run lint` clean.
- (Behavioral register/login/refresh/logout against the live backend is verified by the orchestrator separately.)

## Result
_TBD_
