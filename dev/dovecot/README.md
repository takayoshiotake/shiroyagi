# Dovecot Development Config

Development IMAP account:

- Email address: `dev@example.test`
- IMAP username: `dev@example.test`
- IMAP password: `dev`
- Local app IMAP host: `localhost`
- Local app IMAP port: `2143`
- Compose app IMAP host: `dovecot`
- Compose app IMAP port: `31143`
- Protocol: `IMAP`

`fixtures/dev@example.test/Maildir/new` contains test messages. The
`dovecot-seed` compose service copies them into the `dovecot_data` volume.
