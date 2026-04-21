---
title: Web App Shell
applies_to:
  - web/src/app/**
  - web/src/proxy.ts
depends_on:
  - web/auth-flow.md
last_verified: 2026-04-21
stable: yes
---

# Web App Shell

Next.js app-router marketing + docs + dashboard wrapper. `/` is the
landing, `/docs` and `/security` are static content pages, `/dashboard/*`
is the authed app, and `/login` + `/auth/*` handle the OAuth/device
handshake (see `web/auth-flow.md`). Auth gating is the custom `proxy.ts`
hook, not Next middleware.

## Must know

- **The gating file is `src/proxy.ts`, NOT `middleware.ts`.** This ships
  a patched Next build (see the `web/AGENTS.md` note "this is NOT the
  Next.js you know"). Renaming it to `middleware.ts` will silently stop
  working. The `config.matcher` inside still applies.
- **Matcher scope is tight**: `/dashboard/:path*`, `/login`,
  `/auth/success`. Every other route is public — landing, docs, security,
  OAuth redirects, `/api/*` (proxied to `forged-server`). Adding a new
  protected route requires editing the matcher.
- **`/login?code=` must render for authenticated users.** The proxy
  explicitly skips the "already logged in → /dashboard" redirect when a
  `code` query param is present, so a logged-in user can approve a CLI
  handoff. Do NOT simplify this check.
- **`forged_logged_in` is a non-HttpOnly UI hint.** Set by
  `/api/auth/callback`, cleared by logout. Its absence does NOT log a
  user out — the real gate is the encrypted `forged_session` cookie. UI
  code uses it for "show Log Out vs Sign In" without doing an async
  decrypt.
- **JWT `exp` is checked in `proxy.ts`, not at cookie set time.** An
  expired-on-arrival token still lands in the cookie; the next gated
  request redirects to `/login`. See `web/auth-flow.md`.
- **Root layout is a shared chrome** (fonts, ThemeProvider, Toaster,
  TooltipProvider). The landing page ships its own nav component — do
  not put a global nav in `layout.tsx`.
- **Docs is a single `page.tsx`.** No MDX, no content collections; it's
  a long JSX tree with local `Code` / `CodeBlock` components. Adding a
  doc page means either extending this file or creating `/docs/<slug>/
  page.tsx` and wiring the TOC.
- **Dashboard layout reads the session server-side** via `getSession()`
  and redirects to `/login` on miss — belt-and-braces over the proxy,
  since the proxy only runs on matched routes. Layout redirect is the
  last line of defense if the matcher ever drifts.
- CSP in `next.config.ts` locks `connect-src` to the production API
  origin. Changing `NEXT_PUBLIC_API_URL` without updating CSP breaks
  silently in prod builds only.

## Decisions

- `proxy.ts` over `middleware.ts` is a deliberate fork quirk, not an
  accident. Treat it as the convention here.
- Public-first routing (everything public unless matched) over allow-
  list. The matcher is the single source of gating; new gated routes
  MUST update the matcher AND the dashboard layout's server-side check.
- Landing/docs/security are server components with hand-rolled nav so
  they can ship without pulling dashboard client deps.
