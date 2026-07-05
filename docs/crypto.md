# Crypto

## Envelope Encryption

Mail account IMAP and SMTP passwords are encrypted with envelope encryption.

```text
IMAP password  -> DEK -> AES-256-GCM -> imap_accounts.encrypted_password
SMTP password  -> DEK -> AES-256-GCM -> smtp_accounts.encrypted_password
DEK            -> KEK -> AES-256-GCM -> wrapped_dek
```

One envelope creates one DEK and one `wrapped_dek`. Multiple plaintext values
can be encrypted with that DEK, as long as each encrypted blob uses its own GCM
nonce. `imap_accounts.encrypted_password`, `smtp_accounts.encrypted_password`, and
`wrapped_dek` use the same binary blob format:

```text
version | nonce | ciphertext | tag
```

Current blob format:

- version: 1 byte
- nonce: 12 bytes
- tag: 16 bytes
- wrapped DEK AAD: user_id + mail_account_id
- encrypted field AAD: user_id + mail_account_id + field name

For example, `imap_accounts.encrypted_password` uses the `imap_password` field
name, and `smtp_accounts.encrypted_password` uses the `smtp_password` field
name.

The blob contains the encryption format version, so no separate encrypted
password version column is stored. The KEK version is stored in
`mail_accounts.kek_version` for future KEK rotation.
