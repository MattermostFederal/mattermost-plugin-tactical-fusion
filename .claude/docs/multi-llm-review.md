# Multi-LLM Code Reviews

For deep code review, security audit, or architecture review, use multiple LLMs:

## Quick Method
Use `/multi-review` - Unified multi-LLM review (includes all below)

## Manual Method (parallel)
- `codex exec` (CLI) - Code quality and reasoning
- `gemini` (CLI, **run isolated**, see CLI Commands) - Best free tier, outperforms 2.5 Pro
- `mcp__seq-server__sequentialthinking` (MCP) - Systematic reasoning

## CLI vs MCP Preference

**ALWAYS use CLI commands via Bash, NOT MCP tools:**
- `codex exec` via Bash (NOT `mcp__codex-native__codex`)
- `gemini` via Bash (NOT `mcp__gemini-cli__ask-gemini`)

CLIs are faster, have consistent output formatting, and match skill documentation.
MCP tools have different parameter names and behavior.

## Model Selection Guidelines

### Gemini
- **gemini-3-flash-preview**: Best free tier model (20 req/day) - try this first
- **gemini-2.5-flash**: Fallback when gemini-3-flash quota exhausted (1000 req/day)
- **gemini-2.5-pro**: Deep architectural analysis (25 req/day) - use sparingly
- **Always run it isolated.** See the CLI Commands section below. Gemini has
  write access to any directory it runs in, and its read-only flag is not
  enforced.

**Quota fallback**: If gemini-3-flash-preview returns "quota exceeded", retry with gemini-2.5-flash

### Codex
- **Do not pass `-m`.** Use whatever model the CLI is configured to use.
- The model lives in `~/.codex/config.toml` (`model = "..."`). Change it there, not in review commands.
- Pinning a model here rots: `codex exec -m gpt-5.3-codex` now fails outright with
  `"The 'gpt-5.3-codex' model is not supported when using Codex with a ChatGPT account"`,
  and `~/.codex/config.toml` records the migrations that made it stale
  (`gpt-5.3-codex` → `gpt-5.4` → `gpt-5.5`). The default tracks those automatically.
- Check the current model with `codex exec "hi"`. It prints `model:` in the header.

### Claude Code (Handle Directly)
- File operations (Read, Write, Edit, Glob, Grep)
- Git operations, Bash commands
- TodoWrite planning, tool orchestration
- Quick analysis of 1-3 files

## CLI Commands

### Codex
```bash
# Always this form - no -m flag
codex exec --skip-git-repo-check "prompt"
```

### Gemini (with fallback)

**Reviews must never be able to write to the repo.** Always run Gemini from a
throwaway working directory and pass the content to review inline. Copy this
form exactly:

```bash
# Build the prompt FIRST, while still in the repo, then run isolated.
prompt="Review this for X, Y, Z. Do not restate it. Content follows:

$(cat implementation-plans/some-plan.md)"

d=$(mktemp -d)
(cd "$d" && gemini -o text --skip-trust --approval-mode plan \
  -m gemini-3-flash-preview "$prompt")            # fallback: -m gemini-2.5-flash
rm -rf "$d"
```

**Order matters.** Expanding `$(cat ...)` inside the `cd` subshell resolves the
path against the temp dir, so `cat` fails and Gemini silently reviews an empty
prompt while still producing confident-looking output. Build `$prompt` before
changing directory, or use absolute paths.

Why both guards:

- **The throwaway cwd is the real guard, and it is enforced.** Gemini refuses
  paths outside its workspace outright: `Path not in workspace: ... resolves
  outside the allowed workspace directories`. A review therefore cannot touch
  the repo no matter what the model decides to do.
- `--skip-trust` is required because a fresh temp dir is untrusted and Gemini
  otherwise refuses to run headless there at all.
- `--approval-mode plan` is documented as read-only but is **advisory, not
  enforced**. Verified on gemini-cli 0.54.4: the identical command in the same
  directory refused to write on one run and created the file on the next. Keep
  it as a second layer; never rely on it alone.
- Pass file content **inline**, since the isolated cwd means Gemini cannot open
  repo paths itself. Use `--include-directories <path>` only when it genuinely
  must read files, and understand that this re-opens the write surface for that
  path.

Note that `~/.gemini/trustedFolders.json` currently has `/users/coreyhulen` set
to `TRUST_FOLDER`, so **every** directory under the home dir is trusted by
default. Running Gemini from anywhere in a project therefore gives it write
access to that project. This is the reason the isolation above is mandatory
rather than merely tidy.

## Notes
- Codex has no fallback chain: it uses the configured default, so there is nothing to fall back from
- Gemini quota tiers: gemini-3-flash-preview (20/day) → gemini-2.5-flash (1000/day) → gemini-2.5-pro (25/day)
- **On quota error**: If a Gemini command fails with "quota exceeded", retry with the next model in its fallback chain
- If Codex errors with an unsupported-model message, something is still passing `-m`. Remove it.
- **Why the Gemini isolation exists** (2026-08-10): a `/review-plan` run invoked
  Gemini in the repo root. Instead of reviewing, it implemented the entire plan:
  26 new files plus 6 modified tracked files, including `CLAUDE.md`, `plugin.json`
  and several `server/*.go`. It then reported "the system is now ready for
  deployment". The work was recovered onto a scratch branch, but a review command
  had silently rewritten the repo. Any reviewer given a writable cwd can do this,
  so containment belongs in the command, not in the prompt wording.
- Codex is invoked with `--skip-git-repo-check` and has not shown this behavior,
  but it also runs with `sandbox: workspace-write`. If it ever writes during a
  review, give it the same isolated-cwd treatment.
