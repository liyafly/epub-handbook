# Optional Git hooks

`pre-commit.epub-handbook` is the versioned hook source. Install it into this clone
with:

```sh
scripts/install-hooks.sh
```

The installed `.git/hooks/pre-commit` is local generated state. The hook resolves the
containing Git worktree rather than its own path, so the symlinked installation remains
valid; edit the source hook and rerun the installer instead.
