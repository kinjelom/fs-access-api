# fs-access-api

Administrative API for managing filesystem access entities such as users and groups.

API Documentation: [openapi.yaml](internal/app/docs/openapi.yaml)

This server implements the REST API contract (the endpoints tagged `Authz`) required by [ProFTPD mod_auth_rest](https://github.com/kinjelom/proftpd_mod_auth_rest).

## Dev

This project follows an **OpenAPI-first** approach.
Development workflow looks like this:

1. **Design first** – update [`openapi.yaml`](internal/app/docs/openapi.yaml) to define or change your API (endpoints, schemas, security).
2. **Generate code** – run [`oapi-codegen`](oapi-codegen.sh) to regenerate the Go client/server interfaces and client/types from the spec.
3. **Implement interfaces** – implement the generated interfaces using your business logic.

This way, the OpenAPI contract is always the single source of truth, and the Go code is guaranteed to stay in sync.

Prerequisites: https://github.com/oapi-codegen/oapi-codegen

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
```

## Public endpoints tests

```bash

$ curl http://127.0.0.1:8080/api/health
{"banner":"fs-access-api with inmem test config","healthy":true,"reason":null,"started_at":"2025-10-15T09:35:33.030442806Z","uptime_sec":12}

$ curl http://127.0.0.1:8080/api/secret
{"base64url":"exg_FFo7U55h7Nn4dWxb6sBeDfGJ0rXJkq2jIDMCQJk","hex":"7b183f145a3b539e61ecd9f8756c5beac05e0df189d2b5c992ada32033024099","size_bytes":32}

$ curl -X POST http://127.0.0.1:8080/api/crypto/hash -H "Content-Type: application/json" -d '{"algorithm": "raw-md5", "plaintext": "password" }'
"algorithm":"raw-md5","hash":"5f4dcc3b5aa765d61d8327deb882cf99"}

$ curl -X POST http://127.0.0.1:8080/api/crypto/hash -H "Content-Type: application/json" -d '{"algorithm": "crypt-sha512", "rounds": 5000, "saltLen": 16, "plaintext": "password" }'
{"algorithm":"crypt-sha512","hash":"$6$rounds=5000$xarSJespxoKZmCkj$uNlJcyHTRx1KEVabztXFDfoDFsBM38LfRCwKvKvjEplxNAxZ8R4tHH9UsBHZ/Vv/rjgRiOXSjSj.aOOywbvbi1"}

$ curl -X POST http://127.0.0.1:8080/api/crypto/verify -H "Content-Type: application/json" -d '{ "plaintext": "password", "hash":"$6$rounds=5000$xarSJespxoKZmCkj$uNlJcyHTRx1KEVabztXFDfoDFsBM38LfRCwKvKvjEplxNAxZ8R4tHH9UsBHZ/Vv/rjgRiOXSjSj.aOOywbvbi1" }'
{"detected_algorithm":"crypt-sha512","error":null,"verified":true}
```

## Secured users endpoints tests

### Without Authorization

```bash
$ curl http://127.0.0.1:8080/api/health
{"healthy":true,"started_at":"2025-09-24T23:11:23.759988304Z","uptime_sec":1241}

$ curl http://127.0.0.1:8080/api/users
{"code":"Unauthorized","message":"missing 'Authorization' header"}
```

### With valid auth headers

```bash
export BASE_URL="http://localhost:8080"
export API_KEY_ID="key1"
export API_KEY_SECRET="77f280ba374a80132dfe7ddaba5af72476be5ba34477448fff901ebc804e4b1e" # hex
```

#### Bearer Token

```bash
curl -sS "${BASE_URL}/api/users" -H "X-Api-Key: $API_KEY_ID" -H "Authorization: Bearer $API_KEY_SECRET" | jq
```

#### HMAC

```bash
function calculate_hmac() {
    local KEY_SECRET="$1"
    local METHOD="$2"
    local PATH_WITH_QUERY="$3"
    local BODY="$4"

    export HMAC_TS
    export HMAC_BODY_HASH
    export HMAC_SIG

    # timestamp RFC3339 (UTC)
    HMAC_TS="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
    # body hash
    HMAC_BODY_HASH="$(printf "%s" "$BODY" | openssl dgst -sha256 -hex | awk '{print $2}')"
    # canonical string
    # METHOD \n PATH_WITH_QUERY \n TIMESTAMP \n SHA256_HEX(body)
    local CANONICAL="$(printf "%s\n%s\n%s\n%s" "$METHOD" "$PATH_WITH_QUERY" "$HMAC_TS" "$HMAC_BODY_HASH")"
    # signature HMAC-SHA256 in hex
    HMAC_SIG="$(printf "%s" "$CANONICAL" | openssl dgst -sha256 -mac HMAC -macopt "hexkey:${KEY_SECRET}" -hex | awk '{print $2}')"
}

```

Query:

```bash
calculate_hmac "$API_KEY_SECRET" "GET" "/api/users" ""; echo "TS: $HMAC_TS, BODY_HASH: $HMAC_BODY_HASH, SIG: $HMAC_SIG"

# query
curl -sS "${BASE_URL}/api/users" -H "X-Api-Key: $API_KEY_ID" -H "X-Timestamp: $HMAC_TS" -H "X-Content-Sha256: $HMAC_BODY_HASH" -H "Authorization: HMAC $HMAC_SIG" | jq
```

Response:

```json
{
  "items": [{}]
}
```

Query after `security.window_seconds: 60` seconds:

```bash
{
  "code": "Unauthorized",
  "message": "timestamp outside allowed window"
}
```

Query with new the signature:

```bash
calculate_hmac "$API_KEY_SECRET" "GET" "/api/users" ""; curl -sS "${BASE_URL}/api/users" -H "X-Api-Key: $API_KEY_ID" -H "X-Timestamp: $HMAC_TS" -H "X-Content-Sha256: $HMAC_BODY_HASH" -H "Authorization: HMAC $HMAC_SIG" | jq
```

## User Access

Example group definitions:

| Groupname | GID  | Relative Group Home | Chmod Spec        |
|-----------|------|---------------------|-------------------|
| default   | 2000 | default             | `u=rwx,g=rwxs,o=` |
| group-a   | 2001 | a                   | `u=rwx,g=rwxs,o=` |
| group-b   | 2002 | b                   | `u=rwx,g=rwxs,o=` |

Example user definitions:

| Username       | UID  | Groupname | Relative User Home | Calculated Absolute Home Path |
|----------------|------|-----------|--------------------|-------------------------------|
| **operator-a** | 2001 | group-a   | .                  | ${root}/a                     |
| user-a1        | 2002 | group-a   | user-a1            | ${root}/a/user-a1             |
| user-a2        | 2003 | group-a   | user-a2            | ${root}/a/user-a2             |
| **operator-b** | 2004 | group-b   | .                  | ${root}/b                     |
| user-b1        | 2005 | group-b   | user-b1            | ${root}/b/user-b1             |
| user-bx1       | 2006 | group-b   | user-bx            | ${root}/b/user-bx             |
| user-bx2       | 2007 | group-b   | user-bx            | ${root}/b/user-bx             |

Notes:
- `${root}` - path defined on server and depends on mounting points of store volume, e.g.: `/store/homes`
