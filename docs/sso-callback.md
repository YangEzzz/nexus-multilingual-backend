# Automatic SSO callback

The backend generates `/api/v1/auth/sso/callback` from its trusted public deployment address at startup:

- Local development and deployments that proxy API requests through the frontend: use `FRONTEND_URL`. For example, `http://localhost:5173` produces `http://localhost:5173/api/v1/auth/sso/callback`.
- Deployments with a separate public API domain: set `PUBLIC_API_URL` to the API origin, for example `https://nexus-api.team-tool.top`. Do not include `/api/v1` in this setting.
- An existing nonempty `HELIOS_REDIRECT_URI` takes precedence for compatibility. Remove it to use automatic generation.

Register each generated full URL in the Helios application's allowed return addresses once. One registration may include local and production URLs; separate client registrations are optional when independent credentials or enablement are desired. This change does not merge existing clients or rotate their credentials.

The callback returns to Nexus. Nexus then redirects to its configured frontend completion page. Both authorization and code exchange use the same resolved URI. Incoming Host, forwarded headers, or caller-provided redirect URLs cannot change it. Helios continues exact allowlist validation; CSRF, transaction ownership and PKCE checks are unchanged.

This is deployment-based configuration, not unrestricted runtime discovery. A changed local port requires updating `FRONTEND_URL` and its registered callback. A separate API domain must be configured because it cannot safely be inferred from the frontend domain. No database migration is required.
