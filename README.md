# HelloAsso Renew contributions management script

## Goal

Hello Asso renew contribution management script which do :

- Gather payments from Hello Asso API.
- Compare and update with association member in baserow.
- Email needed people for contribution renew.

## Configuration

Following env var are required :
- HELLOASSO_API_ID
- HELLOASSO_API_SECRET
- HELLOASSO_FROM_DATE
- HELLOASSO_ORG_SLUG

## Run

### Dev mode

`go run .`

