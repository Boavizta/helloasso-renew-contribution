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
