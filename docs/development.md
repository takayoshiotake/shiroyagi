# Development

```bash
podman compose -f compose.yaml -f compose.dev.yaml up
```

Required secrets:

```text
secrets/dev/postgres_password
secrets/dev/mail_account_kek
secrets/dev/oidc_client_secret
```

## UI Development

When creating or modifying UI, templates, or CSS, follow
[`design.md`](design.md).

Reuse existing layouts and components before introducing new patterns.

## Branch Naming

For work associated with a GitHub issue, include the issue number in the branch
name:

```text
<type>/<issue-number>-<short-description>
```

Examples:

```text
docs/14-design-guidelines
feat/13-mail-ui
fix/25-login-redirect
```

The issue number may be omitted for work that is not associated with an issue.

## Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
specification:

```text
<type>[optional scope]: <description>
```

Use `feat` for new functionality, `fix` for bug fixes, and an appropriate type
such as `docs`, `refactor`, `test`, or `chore` for other changes. Write the
description in the imperative mood and keep it concise.

Examples:

```text
feat(mail): add message list
fix(auth): preserve redirect after login
docs: add UI design guidelines
```
