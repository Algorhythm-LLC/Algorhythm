# Publishing `github.com/algorhythm/strategy-dsl`

The canonical copy of this module lives in the Algorhythm meta-repo under
`modules/strategy-dsl/`. To publish as a standalone repository (for `go get`):

1. Create an empty repository `algorhythm/strategy-dsl` on your forge.
2. From this directory (or a clone of only this subtree):

   ```bash
   git init
   git add -A
   git commit -m "feat: initial strategy-dsl module (Phase A)"
   git remote add origin git@github.com:algorhythm/strategy-dsl.git
   git branch -M main
   git push -u origin main
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. In meta-repo, optionally replace the tracked directory with
   `git submodule add git@github.com:algorhythm/strategy-dsl.git modules/strategy-dsl`
   after removing the inlined copy (one-time migration).

CI runs on push/PR to `main` via `.github/workflows/ci.yml`.
