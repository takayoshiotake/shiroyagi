# OIDC

Development uses Keycloak.

Production may use external Keycloak or any configured OIDC provider.

Only explicitly configured issuers are allowed.

## Login cookies

Shiroyagi uses short-lived cookies during the OIDC authorization code flow:

- `shiroyagi_oauth_state`: random `state` value created at `/auth/login` and
  checked at `/auth/callback` to prevent login CSRF.
- `shiroyagi_oauth_nonce`: random `nonce` value created at `/auth/login` and
  checked against the ID token `nonce` claim at `/auth/callback`.
- `shiroyagi_session`: Shiroyagi application session created after the ID
  token is verified.
- `shiroyagi_force_login`: short-lived marker set by `/auth/logout`; the next
  `/auth/login` adds `max_age=0` so Keycloak asks for credentials again without
  ending the broader Keycloak SSO session.

The state and nonce cookies are deleted after a successful callback. The
current session store is in-memory and intended only for the early development
stub.

When a user signs out, Shiroyagi records the user's subject and logout time in
the in-memory session store. The next callback for that subject must include an
ID token `auth_time` at or after the recorded logout time. This prevents a plain
SSO cookie from silently recreating the Shiroyagi session after an app-only
logout.
