# Development Guidelines

## Hexagonal architecture

- `internal/domain` owns membership models, matching/expiry policy, update planning, and renewal-email rendering. Keep it pure: pass the current time and email settings as arguments. Dependencies point inward; this package imports neither adapters nor application code, environment access, HTTP, or logging.
- `internal/application` owns the renewal use case and the small interfaces it consumes (`Contributions`, `Members`, `Mailer`). Inject ports, clock, logger, and email settings through the constructor. Fetch all inputs before writes; process independent members after a write failure and return joined errors.
- `services/<service>/<service>.go` contains outgoing HTTP adapters. Translate wire formats and service-specific select IDs into domain vocabulary here. Inject configuration and an HTTP client; keep business decisions in the domain.
- `internal/config` loads and validates environment settings. `main.go` is the composition root: configure logging, load settings, wire adapters/use case, and translate failures into a nonzero exit status.
- Add an interface at the consuming boundary when an external dependency needs substituting. Pure calculations use concrete types and functions.

## Membership invariants

- Accept only the two annual membership forms. Keep the latest contribution per payer email; when several member aliases match, use the newest contribution across those aliases. Keep planning deterministic.
- Paid contributions expire after 12 calendar months; free memberships after 13. The exact boundary remains valid. Free memberships never receive renewal emails.
- Corporate-domain matching activates inactive colleagues without email. Public, alumni, ISP, and disposable domains cannot establish organisational affiliation.
- Send paid-renewal reminders only when the most recent email date across all rows sharing the primary recipient email is strictly older than 14 days. Compare recipient emails case-insensitively with surrounding whitespace removed; contribution matching remains unchanged. Plan at most one delivery attempt per recipient per run, including after delivery/persistence failures. Empty recipients never receive reminders.
- After successful delivery, set the email date and increment each planned row's own count for that recipient. Failed delivery leaves histories unchanged; persist membership changes even if email fails or is throttled. Read-only/unchanged rows may supply reminder history without requiring an update.
- Plan at most one update per member from the current contribution data. A stale original snapshot must never undo a renewal within the same run.
- Exclude archived Baserow rows server-side. Keep Baserow field names and numeric select IDs inside its adapter. Serialize missing dates as JSON `null`.

## Configuration and logging

- Read environment variables only in `internal/config`; validate before making external calls. Keep deployment defaults there and make endpoints, sender details, and links overridable. Document settings in `README.md`.
- Preserve `DRY_RUN` protection at both writing adapters: reads still happen, writes and email sends are logged without outgoing requests. Reject malformed booleans at startup.
- Use only `log/slog` for logs, including reconciliation diagnostics, with structured context. The composition root sets the default logger.
- Use `samber/lo` for collection transformations. Ordinary loops are appropriate for ordered side effects and error collection.
- Reuse the injected timeout-bound HTTP client. Close response bodies on all paths. Propagate API, decoding, and transport failures. Avoid automatic retries of email delivery without an idempotency mechanism.

## Unit tests (TU)

- Run `make test` before completing changes; use `make test-cover` for coverage and `go vet ./...` for static checks. Format Go files with `gofmt`.
- Domain: table-driven tests with fixed dates, boundary instants, paid/free policies, aliases, public/corporate domains, email language/escaping, input immutability, and stale-snapshot regressions. Cover duplicate-recipient histories (including unchanged rows), normalization, input order, and single-attempt planning.
- Application: in-memory port fakes and a fixed clock; assert writes, delivery bookkeeping, read-before-write behavior, continuation after failures, and returned errors.
- Adapters: `httptest` servers only, asserting authentication, paths/query filters, pagination, mapping/payloads, error handling, and no write requests during dry runs. Never call production APIs or require real credentials in tests.
- Configuration: inject an environment lookup; test missing/invalid settings and defaults/overrides. Keep tests independent of a developer's `.env`.
- Add a regression test whenever changing a membership rule or fixing a bug. Keep `README.md` and these architecture boundaries aligned with the implementation.
