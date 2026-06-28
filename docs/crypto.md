# Crypto

## Encrypted Password Blob

```text
version | nonce | ciphertext | tag
```

AES-256-GCM:

- nonce: 12 bytes
- tag: 16 bytes
- AAD: user_id + mail_account_id
