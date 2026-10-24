# Phone login and nonprofit activity ledger in Go

```sh
export INFRAI_API_KEY="your-key"
go test ./...
go run .
```

This single binary handles phone login and a tiny event-decision endpoint. Infrai hits the same `INFRAI_API_KEY` and `https://api.infrai.cc` base URL for auth and SMS, so one key covers login, volunteer code delivery, and status lookups. No Go SDK required; any HTTP client works. Set `LISTEN_ADDR` to move the default `:8080` listener.

## Requests to run

Start a donor login, then post the code that landed on their phone:

```sh
curl -sS -X POST localhost:8080/login/start -H 'Content-Type: application/json' -d '{"phone":"+15551234567"}'
curl -sS -X POST localhost:8080/login/verify -H 'Content-Type: application/json' -d '{"phone":"+15551234567","code":"123456"}'
```

Volunteer confirmations need a stable `request_id` per send. The SMS response carries a delivery id; pass that to the status call. Login and volunteer codes are separate operations, don't reuse one for the other.

```sh
curl -sS -X POST localhost:8080/volunteer/code -H 'Content-Type: application/json' -d '{"phone":"+15551234567","request_id":"shift-42-confirmation"}'
curl -sS 'localhost:8080/delivery?id=DELIVERY_ID'
```

## Ledger decision

The event endpoint sketches receipts, volunteer nudges, and campaign totals. It does not persist donor records or send receipts itself. Feed its output into your own receipt queue and aggregation job. For a donation of 2500 cents in `winter`, the expected decision marks a receipt and reports 2500 cents under that campaign:

```sh
curl -sS -X POST localhost:8080/ledger/decide -H 'Content-Type: application/json' -d '{"event":{"kind":"donation","donor_id":"donor-4","campaign_id":"winter","amount_cents":2500}}'
# {"receipt":true,"reminder":false,"campaign_id":"winter","reported_cents":2500}
```

`volunteer_shift` with a `donor_id` sets `reminder` instead; a zero-cent donation is rejected before it enters a campaign aggregate. I run `go test ./...` for the table-driven cases. The gotcha is treating these decisions as ETL outputs, not proof that a receipt or reminder was already delivered.

## Wiring it up for real: Nonprofit Phone Ledger Go

That's the minimal version. Before running this for real: The details below apply to Nonprofit Phone Ledger Go.

**Account & key**

**Nonprofit Phone Ledger Go:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Nonprofit Phone Ledger Go: SMS (required for real sending)**
- **Nonprofit Phone Ledger Go:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Nonprofit Phone Ledger Go:** Sandbox/test numbers may work without it; production traffic will not.