# gc hooks sync — investigation & wiring plan

Upstream: gastownhall/gascity#736 ("feat: add `gc hooks sync` command to
propagate `hooks/claude.json` changes"). Triage: S-M, "Helper exists,
mostly wiring." This plan captures the investigation and a concrete
sketch for implementation.

## Summary

The `hooks.Install` helper already does exactly what the upstream issue
asks for; it's just not exposed as a standalone CLI command. Today the
helper is reachable only through `gc init`, `gc rig add`, or the agent
spawn path in `build_desired_state.go`, so users editing
`hooks/claude.json` have no user-facing way to propagate changes to
`.gc/settings.json` without re-triggering one of those side effects.

A thin cobra wrapper — `gc hooks sync` — resolves this.

## Existing helper

`internal/hooks/hooks.go`:

- `Install(fs fsys.FS, cityDir, workDir string, providers []string) error` (line 77) — entry point; switches on provider and calls the per-provider installer.
- `installClaude(fs, cityDir)` (line 114) — reads `hooks/claude.json`, falls back to `.gc/settings.json`, falls back to embedded default; writes both files; preserves user customizations via `claudeFileNeedsUpgrade` gating.
- `Validate(providers []string) error` (line 47) — rejects unknown / no-hook providers.
- `SupportedProviders() []string` (line 39) — returns the eight supported names.

The helper is idempotent. Running it twice is a no-op on the second
call. `installClaude` already implements the "don't overwrite user
edits" behavior the upstream acceptance asks for.

## Provider resolution

`config.ResolveInstallHooks(cfgAgent, workspace) []string`
(`internal/config/resolve_test.go:655` exercises it) merges workspace
and agent-scoped `install_agent_hooks` config. Call sites:

- `cmd/gc/build_desired_state.go:1040` — per-agent spawn side effect.
- `cmd/gc/cmd_start.go:519, 526` — validates before start.
- `cmd/gc/cmd_supervisor.go:1460, 1466` — validates on supervisor boot.
- `cmd/gc/cmd_rig.go:330` — installs on `gc rig add`.
- `cmd/gc/cmd_init.go:567` — installs `["claude"]` unconditionally on `gc init`.

For `gc hooks sync` the natural behavior is:

1. Load the city config.
2. Union of `Workspace.InstallAgentHooks` and every agent's
   `InstallAgentHooks` → providers to reinstall. Claude is city-wide so
   gets installed once at `cityDir`; per-agent providers (gemini,
   codex, etc.) need to be installed per-agent `workDir`.
3. Alternatively (simpler first cut): take workspace-level providers
   only, default to `["claude"]` if empty. This matches `gc init`'s
   behavior and covers the stated upstream use case (propagating
   `hooks/claude.json` changes). The per-agent variant can land in a
   follow-up if needed.

Recommend shipping the simple workspace-scoped variant first and
iterating — it satisfies the upstream acceptance criteria and keeps
the PR small.

## Cobra wiring

No existing `hooks` parent command. `cmd/gc/cmd_hook.go` is the
singular `gc hook` command (work-query injector) and unrelated beyond
the name.

Registration pattern (`cmd/gc/main.go`):

```
newInitCmd(stdout, stderr),   // line 101
newRigCmd(stdout, stderr),    // line 109
newConfigCmd(stdout, stderr), // line 118
newHookCmd(stdout, stderr),   // line 121
```

Add `newHooksCmd(stdout, stderr)` alongside. Parent-with-subcommand
reference: `cmd/gc/cmd_config.go` (`newConfigCmd` → `show`, `explain`
subcommands with `Args: cobra.NoArgs` and `RunE` returning
`cmd.Help()`). Same shape applies here even with only one subcommand
today, because the issue hints at future siblings (`install`,
`reinstall`, `list`).

## Proposed shape

New file `cmd/gc/cmd_hooks.go`:

```go
func newHooksCmd(stdout, stderr io.Writer) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "hooks",
        Short: "Manage agent hook files",
        Args:  cobra.NoArgs,
        RunE: func(c *cobra.Command, _ []string) error {
            return c.Help()
        },
    }
    cmd.AddCommand(newHooksSyncCmd(stdout, stderr))
    return cmd
}

func newHooksSyncCmd(stdout, stderr io.Writer) *cobra.Command {
    var dryRun bool
    var providersFlag []string
    cmd := &cobra.Command{
        Use:   "sync",
        Short: "Reinstall hook files for the current city",
        Long:  `Propagates hooks/claude.json changes to .gc/settings.json...`,
        Args:  cobra.NoArgs,
        RunE: func(_ *cobra.Command, _ []string) error {
            if cmdHooksSync(providersFlag, dryRun, stdout, stderr) != 0 {
                return errExit
            }
            return nil
        },
    }
    cmd.Flags().StringSliceVar(&providersFlag, "providers", nil,
        "override providers to install (default: from city config)")
    cmd.Flags().BoolVar(&dryRun, "dry-run", false,
        "report what would change without writing")
    return cmd
}

func cmdHooksSync(providers []string, dryRun bool, stdout, stderr io.Writer) int {
    cityPath, err := resolveCity()
    if err != nil {
        fmt.Fprintf(stderr, "gc hooks sync: %v\n", err)
        return 1
    }
    cfg, err := loadCityConfig(cityPath)
    if err != nil {
        fmt.Fprintf(stderr, "gc hooks sync: %v\n", err)
        return 1
    }
    if len(providers) == 0 {
        providers = cfg.Workspace.InstallAgentHooks
        if len(providers) == 0 {
            providers = []string{"claude"}
        }
    }
    if err := hooks.Validate(providers); err != nil {
        fmt.Fprintf(stderr, "gc hooks sync: %v\n", err)
        return 1
    }
    if dryRun {
        fmt.Fprintf(stdout, "would install providers: %s\n",
            strings.Join(providers, ", "))
        return 0
    }
    fs := fsys.OS{}
    if err := hooks.Install(fs, cityPath, cityPath, providers); err != nil {
        fmt.Fprintf(stderr, "gc hooks sync: %v\n", err)
        return 1
    }
    fmt.Fprintf(stdout, "synced providers: %s\n", strings.Join(providers, ", "))
    fmt.Fprintln(stdout, "hint: run 'gc supervisor reload' to pick up changes in a running supervisor")
    return 0
}
```

Register in `main.go` next to `newHookCmd(stdout, stderr)`:

```go
newHooksCmd(stdout, stderr),
```

### Open questions for the implementer

1. **Dry-run fidelity.** The simple version above reports intent
   without reading current state. A richer `--dry-run` would diff
   each `hooks/claude.json` / `.gc/settings.json` against what
   `installClaude` would write. That requires factoring the
   "compute desired content" step out of the current combined
   read-decide-write in `installClaude`. Defer to a follow-up
   unless the reviewer asks for it.

2. **Supervisor reconcile.** Upstream acceptance #4 asks the command
   to trigger a reconcile or document the next step. This plan hard-
   codes a printed hint (`gc supervisor reload`). A real trigger
   would require talking to the supervisor process; that's outside
   "wiring" and should be a separate bead if desired.

3. **Per-agent providers.** The workspace-only variant above ignores
   per-agent `install_agent_hooks`. Revisit if the reviewer flags
   that omission; code would iterate agents and call `hooks.Install`
   with each agent's resolved `workDir`.

## Tests

Pattern: `cmd/gc/cmd_hook_test.go` (pure-function tests against the
`doHook`-style inner function, no cobra `.Execute()`).

New file `cmd/gc/cmd_hooks_test.go` covering:

- `TestHooksSync_DefaultClaude` — empty workspace providers → installs
  `["claude"]`; verifies both `hooks/claude.json` and
  `.gc/settings.json` exist after.
- `TestHooksSync_UsesWorkspaceProviders` — workspace
  `install_agent_hooks=["claude","gemini"]` → both installed.
- `TestHooksSync_ProvidersFlagOverrides` — `--providers gemini` wins
  over workspace config.
- `TestHooksSync_ValidationError` — `--providers bogus` → nonzero
  exit, error mentions supported list.
- `TestHooksSync_Idempotent` — second run is a no-op and both files
  still exist with unchanged content.
- `TestHooksSync_DryRun` — writes nothing but prints provider list.
- `TestHooksSync_PreservesUserEdits` — seed `hooks/claude.json` with
  a custom entry that doesn't trigger `claudeFileNeedsUpgrade`;
  assert it's preserved after sync.

Integration test (build tag `integration`) mirroring
`test/integration/e2e_test.go:TestE2E_Hooks_Claude` pattern: run the
compiled binary against a tmpdir city.

## Primitive test (per engdocs/contributors/primitive-test.md)

1. **Atomicity.** The underlying `hooks.Install` already writes
   idempotently via `writeEmbeddedManaged`; the CLI adds no
   concurrency surface. ✅
2. **Bitter Lesson.** A smarter model doesn't make "propagate
   `hooks/claude.json` to `.gc/settings.json`" unnecessary — it's
   pure filesystem plumbing. ✅
3. **ZFC.** The command contains no judgment calls — it resolves
   providers from config, validates, and calls the installer. All
   transport, no cognition. ✅

Passes. Ship it.

## Out of scope

- Actual supervisor reload integration (hint-only for v1).
- Per-agent provider iteration (workspace-level only for v1).
- Diff-quality `--dry-run`.
- Rename / alias of `hook` vs. `hooks` — they're distinct namespaces.

## PR Description

```
feat: add `gc hooks sync` command to propagate hook file changes

Closes #736.

## Summary

- New `gc hooks` parent with one subcommand, `gc hooks sync`, that
  reinstalls provider hook files for the current city. The default
  provider list comes from `workspace.install_agent_hooks` (falling
  back to `["claude"]`). `--providers` overrides the config list;
  `--dry-run` reports the resolved providers without writing.
- Thin cobra wrapper over the existing `hooks.Install` helper that
  `gc init`, `gc rig add`, and the agent spawn path already use.
  Idempotent — a second run is a no-op.
- Prints a hint to run `gc supervisor reload` so a live supervisor
  picks up the new `.gc/settings.json`.

## Scope (intentional for v1)

- Workspace-level providers only. Per-agent `install_agent_hooks`
  iteration is deferred; claude is city-wide and covers the stated
  upstream use case.
- `--dry-run` reports intent without diffing file content. A diff-
  quality dry-run would require factoring the "compute desired
  content" step out of `installClaude`; deferred.
- No supervisor-reload trigger — printed hint only.

## Test plan

- [x] Unit tests in `cmd/gc/cmd_hooks_test.go` cover: default claude,
  workspace providers, `--providers` override, validation error,
  idempotent second run, `--dry-run`, user-edit preservation, error
  surfacing via injected FS, and missing-city error.
- [x] `make check` passes (fmt, lint, vet, unit tests).
- [x] `make check-docs` passes.
```

Reviewer pre-flight (for my own use — delete before publishing):

- PR is ~100 LOC over 3 files (`cmd/gc/cmd_hooks.go`, `cmd/gc/cmd_hooks_test.go`, `cmd/gc/main.go` one-line registration), within the <200/≤5 sweet spot.
- Help strings and doc comments match the actual behavior (workspace-scoped, claude default, idempotent, hint-only reload).
- Tests assert on specific substrings (`"synced providers: claude"`, `"would sync providers: ..."`, `"supported:"`) rather than "some output produced."
- Edge cases covered: unsupported provider name (validation), injected FS write error (error path), missing city directory (load error), user-edited `hooks/claude.json` preserved.
- No backward-compat changes: this is an additive command; existing `gc hook` (singular) is unaffected.
- Sibling-API consistency: `newHooksCmd` / `newHooksSyncCmd` follow the `newConfigCmd` / `newConfigShowCmd` parent-with-subcommands shape; `cmdHooksSync` / `doHooksSync` follow the `cmdAgentAdd` / `doAgentAdd` CLI-vs-pure split (pure function takes `fs fsys.FS` for testability).
