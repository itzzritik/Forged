---
title: Web App Shell
applies_to:
  - web/src/app/**
  - web/src/proxy.ts
depends_on:
  - web/auth-flow.md
last_verified: 2026-04-23
stable: yes
---

# Web App Shell

The web app is a public marketing/docs shell plus an authenticated dashboard.

## Must know

- Auth gating lives in `src/proxy.ts`, not `middleware.ts`.
- `/login?code=...` must still render for already-authenticated users so they can approve CLI login.
- Dashboard layout also checks session server-side; the proxy is not the only gate.
- Public pages stay public unless the proxy matcher says otherwise.
- CSP still matters for API calls; changing the API origin without updating CSP will break auth and dashboard traffic.

## Decisions

- Keep the custom `proxy.ts` convention used by this app.
