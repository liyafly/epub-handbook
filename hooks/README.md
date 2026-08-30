# Optional Git hooks

`pre-commit.epub-handbook` is the versioned hook source. Install it into this clone
with:

```sh
cp hooks/pre-commit.epub-handbook .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit
```

The installed `.git/hooks/pre-commit` is local generated state. The hook resolves the
containing Git worktree rather than its own path; edit the source hook and reinstall
with the command above. The hook only needs a Go toolchain: it runs the architecture
guards and the `epub` CLI on the style-demo artifact.
