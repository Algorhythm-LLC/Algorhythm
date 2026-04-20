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

3. **Meta-repo: replace inlined copy with submodule** (do this soon after the
   remote exists, so two diverging trees do not linger):

   ```bash
   cd /path/to/algorhythm/meta
   git rm -rf modules/strategy-dsl
   git commit -m "chore: remove inlined strategy-dsl before submodule"
   git submodule add git@github.com:algorhythm/strategy-dsl.git modules/strategy-dsl
   git submodule update --init --recursive
   git commit -m "chore: add strategy-dsl as git submodule"
   ```

4. In `services/control-plane`, remove the temporary line in `go.mod`:

   ```text
   replace github.com/algorhythm/strategy-dsl => ../../modules/strategy-dsl
   ```

   after `go get github.com/algorhythm/strategy-dsl@v0.1.0` succeeds (module
   resolvable from the proxy).

CI runs on push/PR to `main` via `.github/workflows/ci.yml`.
