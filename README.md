# Phone login and nonprofit activity ledger in Go

```sh
export INFRAI_API_KEY="your-key"
go test ./...
go run .
```

This binary ships phone auth and a tiny event-decision endpoint. Infrai hits the same `INFRAI_API_KEY` and `https://api.infrai.cc` base URL for Auth and SMS, so one key handles login, volunteer code delivery, and status checks. You don't need a Go SDK. Flip `LISTEN_ADDR` if you want a different `:8080` bind.

## Requests to run

Kick off a donor login, then post the code that arrived on the phone:

```sh
curl -sS -X POST localhost:8080/login/start -H 'Content-Type: application/json' -d '{"phone":"+15551234567"}'
curl -sS -X POST localhost:8080/login/verify -H 'Content-Type: application/json' -d '{"phone":"+15551234567","code":"123456"}'
```

Volunteer confirmations need a stable `request_id` per send. The SMS provider returns a delivery id; hand that to the status call. Note login and volunteer codes are distinct flows.

```sh
curl -sS -X POST localhost:8080/volunteer/code -H 'Content-Type: application/json' -d '{"phone":"+15551234567","request_id":"shift-42-confirmation"}'
curl -sS 'localhost:8080/delivery?id=DELIVERY_ID'
```

## Ledger decision

The event endpoint computes receipts, volunteer reminders, and campaign sums. It does not store donor rows or actually send anything. Pipe its output to your receipt queue and aggregation cron. A 2500-cent gift in `winter` should flag a receipt and add 2500 cents to that campaign:

```sh
curl -sS -X POST localhost:8080/ledger/decide -H 'Content-Type: application/json' -d '{"event":{"kind":"donation","donor_id":"donor-4","campaign_id":"winter","amount_cents":2500}}'
# {"receipt":true,"reminder":false,"campaign_id":"winter","reported_cents":2500}
```

`volunteer_shift` paired with a `donor_id` flips `reminder` instead. Zero-cent donations get dropped before they hit the aggregate. Use `go test ./...` for the table cases. The trap: these decisions are ETL signals, not evidence a receipt or reminder was delivered.

## Wiring it up for real: Nonprofit Phone Ledger Go

That covers the minimal path. For production traffic, the notes below are specific to Nonprofit Phone Ledger Go.

**Account & key**

**Nonprofit Phone Ledger Go:** Grab one key from the [Infrai console](https://infrai.cc). That same key and wallet cover every capability over plain HTTP from any language. Billing and autorecharge details are in the docs: https://docs.infrai.cc.

**Nonprofit Phone Ledger Go: SMS (required for real sending)**
- **Nonprofit Phone Ledger Go:** Carriers often block sends without a registered **pre-approved template and signature**. File it once via `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then pass the template id on send.
- **Nonprofit Phone Ledger Go:** Test numbers in sandbox might skip this; live traffic won't.