# Conventional Commits

All commit messages MUST follow the Conventional Commits specification.

## Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

## Types

| Type       | Purpose                                                 |
| ---------- | ------------------------------------------------------- |
| `feat`     | A new feature (MINOR in SemVer)                         |
| `fix`      | A bug fix (PATCH in SemVer)                             |
| `docs`     | Documentation only changes                              |
| `style`    | Formatting, whitespace — no code logic changes          |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `perf`     | Performance improvement                                 |
| `test`     | Adding or correcting tests                              |
| `build`    | Build system or dependency changes                      |
| `ci`       | CI configuration changes                                |
| `chore`    | Other changes that don't modify src or test files       |
| `revert`   | Reverts a previous commit                               |

## Rules

- Type MUST be lowercase: `feat`, not `Feat`
- Use imperative mood: "add feature" not "added feature"
- No period at end of description
- Description follows colon + space: `feat: add login`
- Limit description to 50-72 characters
- Scope is optional, must be a noun: `feat(auth): add OAuth`
- Body and footers separated by blank lines
- Breaking changes: append `!` after type/scope (`feat!:`) or add `BREAKING CHANGE:` footer
