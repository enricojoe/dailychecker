# M8 — Frontend: Activities, Today, History, Telegram

> Execution plan for Milestone 8. Parent plan: [dailychecker.md](dailychecker.md) §Milestone 8.

## Goal
Build the real feature pages on top of the M7 shell: Activity CRUD (with schedule editor + sub-activities), the Today tri-state checklist with rollup, History (calendar + by-activity), and the Telegram connect flow. End-to-end: create activity → appears in Today → check/partial/done with rollup → shows in both history views → connect Telegram.

## Existing scaffold (M7 — reuse, extend, don't rebuild)
- `apiClient.{get,post,patch,delete}` with Bearer + single-flight 401→refresh→retry; throws `ApiError {status, message}` (message = backend `{error}`). Feature hooks call this.
- TanStack Query v5 (`QueryClientProvider` in `main.tsx`: queries throwOnError true, retry 1, staleTime 60s; mutations throwOnError false).
- `AuthProvider`/`useAuth()` → `{ user, isAuthenticated, login, register, logout }`; `user` (from `GET /api/me`) includes `telegram_chat_id?`, `telegram_linked_at?`.
- Router `src/routes/index.tsx`: protected routes nest under `<AppShell>`. `AppShell` nav already lists Today (`/`), Activities (`/activities`), History (`/history`), Telegram (`/telegram`) — items flagged `m8:true` are disabled placeholders; **enable them as NavLinks** as pages land.
- shadcn/base-ui primitives: `button`, `input`, `label` (+ a `card`/small primitives may have been added). Tailwind v4, `@` alias.

## Backend API contract (built & tested; error envelope `{ "error": msg }`)
Activities (all Bearer-protected):
- `GET /api/activities` → `ActivityNode[]` tree. `ActivityNode = Activity & { children: ActivityNode[] }`.
- `Activity = { id, user_id, parent_id?: string, title, notes?: string, freq: "daily"|"weekly", days_of_week: number[] (0=Sun..6=Sat), time_of_day: "HH:MM:SS", sort_order: number, is_active: boolean, created_at, updated_at }`.
- `POST /api/activities` body `{ parent_id?, title, notes?, freq, days_of_week?, time_of_day, sort_order?, is_active? }` → `201 Activity`.
- `PATCH /api/activities/:id` body (all optional) `{ title?, notes?, freq?, days_of_week?, time_of_day?, sort_order?, is_active?, parent_id? }` → `200 Activity`.
- `DELETE /api/activities/:id` → `204` (cascades to children + occurrences).
- Validation (→ 422 `{error}`): freq daily|weekly; weekly requires non-empty days_of_week (0–6, no dups); daily requires empty days_of_week; time_of_day "HH:MM" or "HH:MM:SS"; single-level depth (parent must be top-level; sub-activity can't get children); cross-user → 404.

Today / occurrences:
- `GET /api/today` → `OccurrenceNode[]` tree. `OccurrenceNode = { id, activity_id, title, state: "pending"|"partial"|"done", completed_at?: string, children: OccurrenceNode[] }`. (Server generates today's occurrences idempotently on each call.)
- `PATCH /api/occurrences/:id` body `{ state }` → `OccurrenceNode[]` — **the updated group tree (parent + children) after rollup**. Use this to reconcile cache.

History:
- `GET /api/history/calendar?from=YYYY-MM-DD&to=YYYY-MM-DD` → `DaySummary[] = { date: "YYYY-MM-DD", pending, partial, done, total }[]` (days with no occurrences are omitted). Range > 365 days → 422; from>to → 422.
- `GET /api/history/calendar/:date` (`YYYY-MM-DD`) → `OccurrenceNode[]` tree for that day.
- `GET /api/history/activities/:id?from=&to=` → `ActivityHistoryEntry[] = { date: "YYYY-MM-DD", state }[]`. Cross-user → 404.

Telegram:
- `POST /api/telegram/link` → `{ url, token }`. **Note:** this route exists ONLY when the backend has a bot token configured; otherwise it 404s. Handle 404 gracefully ("Telegram not configured on the server"). Connected state comes from `useAuth().user.telegram_chat_id`/`telegram_linked_at`.

## Tasks (organize as feature folders under `src/features/*`, each with an `api.ts` + query/mutation hooks + components)
- [ ] **Dates util:** a small Jakarta date helper (format `YYYY-MM-DD`, month grid math). All date params are Jakarta calendar dates as strings.
- [ ] **Activities** (`features/activities/`): typed api + hooks (`useActivities`, `useCreateActivity`, `useUpdateActivity`, `useDeleteActivity`). Page at `/activities`: tree list (parents with nested children), create/edit form with a **schedule editor** (freq toggle daily/weekly; weekday multi-select shown only for weekly; time picker; notes; is_active; sort_order), **add sub-activity** action available only on top-level activities (enforce single-level depth in UI), delete with confirmation. Invalidate `activities` (and `today`) on mutations. Surface 422 messages.
- [ ] **Today** (`features/today/`): replace `TodayPlaceholder`. `useToday` query (`GET /api/today`); render the occurrence tree with expand/collapse children and a **tri-state control** (pending → partial → done) per node. `useSetOccurrenceState` mutation (`PATCH /api/occurrences/:id`): **optimistic** update of the clicked node, then reconcile with the returned group tree (authoritative rollup — parent flips when children change, etc.); rollback + invalidate `today` on error. Empty state when nothing is due.
- [ ] **History** (`features/history/`): page at `/history` with two views (tabs or sub-routes): (1) **Calendar** — month grid; fetch `?from=&to=` for the visible month; each day cell shows status from `DaySummary` (e.g. done/total or a color); clicking a day loads `calendar/:date` detail (occurrence tree, read-only). Month prev/next nav. (2) **By-activity** — pick an activity (from `useActivities`) + a date range → timeline/streak of states from `history/activities/:id`.
- [ ] **Telegram** (`features/telegram/`): page at `/telegram`. If `user.telegram_chat_id` set → show "Connected" + linked_at. Else a "Connect Telegram" button → `POST /api/telegram/link` → show the `url` as a clickable deep-link (and/or QR) with instructions; poll/refetch `me` so the UI flips to Connected after the user links. Handle 404 (telegram disabled on server) gracefully.
- [ ] **Wire routes + nav:** add `/activities`, `/history`, `/telegram` under `<AppShell>` in `routes/index.tsx`; enable the corresponding `AppShell` nav links (drop the disabled placeholders).
- [ ] Loading/empty/error states for every page (use the shell's patterns); accessible controls (keyboard-operable tri-state, labelled form fields).

## DoD (parent plan)
End-to-end: create activity → appears in Today → check/partial/done with rollup reflected → shows in both calendar and by-activity history → connect Telegram. Plus `npx tsc --noEmit`, `npm run build`, `npm run lint` all clean.

## Constraints
- Idiomatic modern React (function components, hooks), TS strict, accessible. Match M7 conventions (feature folders, `@` alias, TanStack Query hooks calling `apiClient`, shadcn/Tailwind). Keep components focused.
- Reflect backend validation in the UI (weekly⇔days_of_week; single-level depth) but rely on the server as source of truth; surface `{error}` messages.
- No new heavy deps without need (react-router + TanStack Query already present; a tiny QR lib is OPTIONAL — if you want one, STOP and tell the orchestrator to install it; do NOT run npm install yourself).
- Don't touch the backend. Don't commit — leave changes for orchestrator review.

## Verification (subagent runs — all local, no network; must be clean)
- `npx tsc --noEmit`, `npm run build`, `npm run lint`.
- (Live integration against the running backend — Today/rollup, activities CRUD, history — is verified by the orchestrator.)

## Result
_TBD_
