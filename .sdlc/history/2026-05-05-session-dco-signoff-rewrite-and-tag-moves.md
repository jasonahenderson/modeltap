# 2026-05-05 — DCO Sign-off Rewrite and Tag Moves

## Context

PR #1 (`spike/scrolling-surface-eval` → `main`) failed the DCO Sign-off
Check on first push because 12 commits in the 308-commit history did not
carry a `Signed-off-by:` trailer. Fixing this required adding the trailer
to those commits, which cascades through descendants and rewrites all
SHAs on the branch.

## Why filter-branch, not rebase

`git rebase --signoff main` aborted at commit 70/304 with a content
conflict in `internal/provider/truncate.go`. The branch contains four
historical worktree-agent merges (`8c18a30`, `4631732`, `88cff03`,
`571d72f`) from parallel agent work; linearizing them via `rebase`
collides with later linear commits that touch the same files.

`git rebase --rebase-merges --signoff main` hit the same conflict at
89/323 because the conflicting content is in the merge content itself,
not in the linearization.

`git filter-branch --msg-filter` rewrites commit messages without
re-merging. It rebuilds the DAG using the original tree contents at each
commit — no merge replay, no conflicts. Tree identity was confirmed
post-rewrite: the rewritten lockdown commit has tree `d48080f5...`,
identical to the pre-rewrite lockdown commit.

## What ran

```
git filter-branch -f --msg-filter '
  msg=$(cat)
  if printf "%s" "$msg" | grep -q "^Signed-off-by:"; then
    printf "%s" "$msg"
  else
    printf "%s\n\nSigned-off-by: %s <%s>" "$msg" \
      "$(git config user.name)" "$(git config user.email)"
  fi
' main..HEAD
```

Branch HEAD: `9cedf88` → `aada034`.

## Pre-rewrite anchor

`pre-dco-rewrite` annotated tag at `9cedf88c81c8ea8c14b4ba707294c558dabe72ef`,
created and pushed before the rewrite. Acts as the rollback anchor.
Rollback procedure if needed:

```
git reset --hard pre-dco-rewrite
git tag -f design-locked-v0.3.x cbf5862ef8ec30a758d781e0ff6cfeda296c7880
git tag -f v0.2.1 d0a70b3...
git tag -f v0.2.2 7b7f6bb...
git push --force-with-lease origin spike/scrolling-surface-eval
git push --force-with-lease origin refs/tags/design-locked-v0.3.x \
  refs/tags/v0.2.1 refs/tags/v0.2.2
```

## Tag moves

Per `.agents/process.md` § Release Tags ("Any tag move must be logged in
`.sdlc/history/` with the old SHA, new SHA, and reason"):

| Tag | Old SHA | New SHA | Reason |
|---|---|---|---|
| `design-locked-v0.3.x` | `cbf5862ef8ec30a758d781e0ff6cfeda296c7880` | `41b56fef744ec8a2a1003c89856d92992b0889e2` | Re-pin to rebased equivalent commit after DCO sign-off rewrite. Tree identity preserved. |
| `v0.2.1` | `d0a70b3...` (prior release-close commit) | `395c2f78b49febbc354f728fa72031f6a1852ae7` | Re-pin to rebased equivalent commit after DCO sign-off rewrite. Tree identity preserved. |
| `v0.2.2` | `7b7f6bb...` (prior release-close commit) | `4c713f288370811a2d8886ac318ccca0af15c1e1` | Re-pin to rebased equivalent commit after DCO sign-off rewrite. Tree identity preserved. |

Both `v0.2.1` and `v0.2.2` were pushed to `origin` for the first time
earlier in this session (the remote had no `main` and no release tags
prior). They were not yet "published or announced" in the sense of
`.agents/process.md`, so moving them before any external use is allowed
under the tag-update policy.

## Force-push

After the rewrite:

- `git push --force-with-lease origin spike/scrolling-surface-eval`
- `git push --force-with-lease origin refs/tags/design-locked-v0.3.x`
- `git push --force-with-lease origin refs/tags/v0.2.1`
- `git push --force-with-lease origin refs/tags/v0.2.2`

PR #1 picks up the new branch HEAD automatically; the diff against `main`
is unchanged (tree identity preserved), only commit SHAs differ.

## Follow-up

- Watch DCO Sign-off Check turn green on the rewritten PR.
- Lint already fixed in the rewritten equivalent of `9cedf88` (now
  `aada034`).
- Test failures (`TestHandleTurnSubmit_HappyPath`,
  `internal/harness` SIGSEGV, `TestBash_Timeout`,
  `TestBash_OutputTruncation`) are unaffected by this rewrite and
  remain to be triaged separately.
