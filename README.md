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
- HELLOASSO_FROM_DATE : from what date we need to get helloasso data.
- HELLOASSO_ORG_SLUG : slog of your organization in helloasso
- BASEROW_API_TOKEN
- BASEROW_MEMBER_TABLE_ID : base row id of the member table
- BREVO_API_KEY

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

## Run

### Dev mode

`go run .`


### Build binaries

`make build-all`
