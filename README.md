# HelloAsso Renew contributions management script

## Goal

Hello Asso renew contribution management script which do :

- Gather payments and free memberships from Hello Asso API.
- Compare and update with association member in baserow.
- Email needed people for contribution renew.

## TODO

 - [x] spo 1000 € cotisation
 - [ ] check limit of contribution email
 - [x] without contribution ( Personne Physique - Sans Cotisation, Individual - Free and Personne Morale - Cotisation déjà effectuée)

## Configuration

Following env var are required :
- HELLOASSO_API_ID
- HELLOASSO_API_SECRET
- HELLOASSO_FROM_DATE : from what date we need to get helloasso data.
- HELLOASSO_ORG_SLUG : slog of your organization in helloasso
- BASEROW_API_TOKEN
- BASEROW_MEMBER_TABLE_ID : base row id of the member table
- BREVO_API_KEY
- DRY_RUN (optional) : `true` to log Baserow updates and emails instead of performing them

Settings are validated before any API calls, including in dry-run mode.
`HELLOASSO_FROM_DATE` must use `YYYY-MM-DD`, and `BASEROW_MEMBER_TABLE_ID`
must be a positive integer. Invalid `DRY_RUN` values stop the job.

Optional overrides (the existing deployment values remain the defaults):

| Variable | Default |
| --- | --- |
| `HELLOASSO_API_URL` | `https://api.helloasso.com` |
| `BASEROW_API_URL` | `https://baserow.boavizta.org` |
| `BREVO_API_URL` | `https://api.sendinblue.com` |
| `RENEWAL_SENDER_NAME` | `Boavizta` |
| `RENEWAL_SENDER_EMAIL` | `no-reply@boavizta.org` |
| `RENEWAL_FRENCH_LINK` | `https://www.helloasso.com/associations/boavizta/adhesions/cotisation-annuelle` |
| `RENEWAL_ENGLISH_LINK` | `https://www.helloasso.com/associations/boavizta/adhesions/annual-membership-fee` |

API URLs are base URLs, without query strings or fragments. Outgoing HTTP
requests have a 30-second timeout. The program does not load `.env` itself.

## Architecture

The application follows hexagonal architecture (ports and adapters):

```text
main.go / internal/config       composition and validated environment settings
        |
internal/application           renewal use case; owns outgoing port interfaces
        |
internal/domain                pure models, matching/expiry rules, email rendering

services/helloasso              Contributions adapter
services/baserow                Members adapter
services/brevo                  Mailer adapter
```

The domain and application do not import adapters. `main.go` injects concrete
clients, logger, and clock. Service-specific JSON fields and Baserow select IDs
stay in the adapters.

Planning produces at most one update per member. It accepts the latest
contribution across primary/alternative emails and prevents a stale original
snapshot from deactivating a just-renewed member. Paid memberships have a
12-calendar-month validity period; free memberships have 13 months and never
receive renewal email. Corporate-domain activation excludes public providers.
Reminders require more than 14 days since the last successful send.

Reminder guardrails use the **most recent reminder date across all member rows
sharing a primary recipient email**, including rows that need no membership
update. Recipient comparison ignores case and surrounding whitespace; this does
not change contribution matching. Each recipient gets at most one delivery
attempt per run, even if delivery or persistence fails. Empty recipients are
skipped. After a successful send, every planned update for that recipient records
the same send date and increments its own reminder count. Other member updates
still proceed when a reminder is suppressed or fails.

This is a per-run guard, not a distributed delivery lock: overlapping jobs or a
send followed by failure to persist all associated dates can still cause a later
duplicate. Duplicate records should still be corrected in Baserow.

Read failures stop processing before writes. Email/update failures are collected
while other members continue; the job exits nonzero if any operation failed.
Reconciliation diagnostics use structured logs instead of raw CSV lines.
Missing Baserow dates are sent as JSON `null`.

### Unit tests (TU)

```bash
make test          # all tests with race detection
make test-cover    # coverage.out and per-function coverage
go vet ./...
```

Tests use fixed clocks, in-memory ports, and local `httptest` servers. No
credentials, production API access, or `.env` file are needed. Coverage includes
expiry/rate-limit boundaries, domain/alias matching, renewal regressions,
delivery bookkeeping and failures, configuration, and HTTP adapter contracts.
The unit-test workflow runs formatting, vet, and race-enabled tests on pull
requests and pushes to `main`.

## Base row impact

The project use dedicated field as :
 - AlternativeEmail1 (to manage people who have change email or multiple email)
 - AlternativeEmail2 (to manage people who have change email or multiple email)
 - Active MemberShip ( put to true when payed)
 - Last Payment Date (last payement date found in helloasso)
 - Last Contribution Email Date (last contribution email to request membership payment)
 - Number of Contributions Email (number of email sent to request membership payment)

And reuse :

 - Id
 - Surname
 - First Name
 - Email
 - Country
 - PreferredLanguages
 - MembershipType
 - Archived (boolean, read-only filter: archived members are excluded from the query and never processed nor updated)

## Run

### Dev mode

`go run .`


### Build binaries

`make build-all`

## Scaleway Deployment

The project runs on [Scaleway Serverless Jobs](https://www.scaleway.com/en/serverless-jobs/) (fr-par) with a weekly cron trigger every Monday at 6:00 AM (Europe/Paris).

### One-time setup

```bash
# Requires scw CLI (brew install scw), Docker, and scw login done
export SCW_DEFAULT_PROJECT_ID="<your-project-id>"
./scripts/init-scaleway-job.sh
```

This creates:
- **Container Registry** namespace `boavizta-registry`
- **Docker image** built and pushed to the registry
- **Job definition** with default env vars (placeholder values)
- **Cron trigger** — every Monday at 6:00 AM Europe/Paris

The script is idempotent: re-running it updates the image and skips already-existing resources.

### Configure environment variables

The init script sets **placeholder values** for all secrets. Update them with real values after the first run.

**Via Scaleway Console:**
Jobs → `helloasso-renew-contribution` → Environment Variables

**Via CLI:**
```bash
scw jobs definition update <job-id> \
  environment-variables.HELLOASSO_API_ID="real-value" \
  environment-variables.HELLOASSO_API_SECRET="real-value" \
  environment-variables.HELLOASSO_FROM_DATE="2024-01-01" \
  environment-variables.HELLOASSO_ORG_SLUG="real-org-slug" \
  environment-variables.BASEROW_API_TOKEN="real-value" \
  environment-variables.BASEROW_MEMBER_TABLE_ID="12345" \
  environment-variables.BREVO_API_KEY="real-value"
```

### Test run

```bash
scw jobs definition start <job-id>
```

### CI/CD

Push to `main` triggers `.github/workflows/deploy.yml` which builds and deploys a new image automatically. Manual rollback available via `.github/workflows/rollback.yml` (workflow dispatch with tag input).
