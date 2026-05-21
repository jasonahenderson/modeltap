# Contributing to modeltap

Thank you for your interest in contributing to modeltap. This guide explains how to contribute, what we expect from contributions, and how the review process works.

modeltap is licensed under Apache 2.0 (see [LICENSE](LICENSE)). By contributing, you agree that your contributions will be licensed under the same terms.

## How to Contribute

### 1. Fork and Clone

Fork the repository on GitHub, then clone your fork:

```bash
git clone https://github.com/<your-username>/modeltap.git
cd modeltap
```

### 2. Create a Feature Branch

Create a branch from `main` for your work:

```bash
git checkout -b feature/your-feature-name
```

Use a descriptive branch name. Prefixes like `feature/`, `fix/`, or `docs/` help categorize the change.

### 3. Make Your Changes

Follow the project's Architecture Decision Records (ADRs) in `.sdlc/adr/` when making changes. If your change conflicts with an existing ADR, open an issue to discuss it before submitting a PR.

### 4. Sign Off Your Commits (DCO)

All commits must include a Developer Certificate of Origin (DCO) sign-off. This certifies that you have the right to submit the work under the project's Apache 2.0 license.

Add the sign-off by using the `-s` flag:

```bash
git commit -s -m "Add support for new provider"
```

This appends the following to your commit message:

```
Signed-off-by: Your Name <your-email@example.com>
```

Make sure your `user.name` and `user.email` in git config match the sign-off. Every commit in a PR must be signed off. The DCO is enforced by a CI check on all PRs.

If you forget to sign off, you can amend your most recent commit:

```bash
git commit --amend -s
```

Or rebase to sign off all commits in a branch:

```bash
git rebase --signoff HEAD~<number-of-commits>
```

### 5. Submit a Pull Request

Push your branch to your fork and open a pull request against `main`.

### 6. Address Review Feedback

Respond to review comments, push updates to your branch, and maintain the DCO sign-off on all new commits.

## Coding Standards

### Go Code

- **Formatting:** All Go code must pass `gofmt`. Run `gofmt -w .` before committing.
- **Linting:** Code must pass `go vet ./...` with no warnings.
- **Tests:** Write table-driven tests for new functionality. Follow existing test patterns in the codebase.
- **Naming:** Follow standard Go naming conventions. Exported names should be clear and descriptive.

### General

- Keep commits focused. One logical change per commit.
- Write clear commit messages. The first line should summarize the change in 50 characters or fewer, followed by a blank line and a detailed description if needed.

## Pull Request Requirements

Every pull request must include:

1. **Description:** A clear explanation of what the change does and why it is needed.
2. **ADR reference:** If the change relates to an architectural decision, reference the relevant ADR (e.g., "Implements ADR-0006 provider adapter interface").
3. **Tests:** New functionality must include tests. Bug fixes should include a test that reproduces the bug.
4. **DCO sign-off:** All commits must be signed off.
5. **Passing CI:** All CI checks must pass before a PR is reviewed.

## Code Review Process

1. **Triage:** A maintainer or committer reviews the PR for completeness (description, tests, DCO, CI status).
2. **Review:** One or more reviewers examine the code for correctness, adherence to ADRs, test coverage, and style.
3. **Feedback:** Reviewers leave comments. Contributors address feedback and push updates.
4. **Approval:** A committer (for changes in their area) or maintainer approves the PR.
5. **Merge:** A maintainer merges the PR to `main`.

For architectural changes or changes that affect multiple subsystems, the BDFL (Jason Henderson) reviews and approves.

## Contributor Tiers

modeltap uses a graduated contributor tier system. See [GOVERNANCE.md](GOVERNANCE.md) for full details.

| Tier | Role | How to Get There |
|------|------|-----------------|
| 1 | Contributor | Submit a PR |
| 2 | Committer | 5+ quality merged PRs |
| 3 | Maintainer | Sustained commitment and architectural understanding |
| 4 | BDFL | Project founder |

## Reporting Issues

Open an issue on GitHub. Include:

- A clear description of the problem or feature request
- Steps to reproduce (for bugs)
- Expected vs. actual behavior (for bugs)
- Environment details (Go version, OS, modeltap version)

## Questions

If you have questions about contributing, open a GitHub issue with the `question` label. We are happy to help first-time contributors get started.
