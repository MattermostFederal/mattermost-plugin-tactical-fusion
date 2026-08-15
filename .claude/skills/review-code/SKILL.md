---
name: review-code
description: Comprehensive code review via specialized agents + multi-LLM review. Works on local changes or GitHub PRs.
---

# Review Code

Comprehensive code review using **specialized agents** AND **multi-LLM review**. Catches bugs, security issues, and pattern violations.

Works on:
- **Branch changes since last PR** (default) - commits on the current branch vs `master`, plus uncommitted working-tree changes
- **Uncommitted only** (with `--uncommitted` flag) - working-tree changes only
- **GitHub PRs** (with `--pr` flag)

> **Taxonomy**:
> - `/create-plan` → `/create-code` → `/review-code`
> - This is the final quality gate before committing

**Run `/lint` after** - This skill finds semantic issues; lint afterward to clean up formatting.

**Related**:
- `/review-plan` - Review plans (different agents, different LLM models)
- `/create-code` - Implement code from plan
- `/lint` - Linting and formatting (run first)

## Two-Phase Review

This skill combines:
1. **Specialized Code Agents** - Claude agents for pattern-specific checks
2. **Multi-LLM Review** - External models for diverse perspectives

| Phase | Models/Agents | Strength |
|-------|---------------|----------|
| **Phase 1: Agents** | Claude agents (race-condition-finder, etc.) | Pattern detection, domain-specific |
| **Phase 2: Multi-LLM** | Codex, Gemini, seq-server (see `multi-llm-review.md`) | Code quality, diverse perspectives |

## Usage

```
/review-code                              # All changes on current branch since master (default)
/review-code --uncommitted                # Uncommitted working-tree changes only
/review-code <file-or-directory>          # Review specific path
/review-code --pr 123                     # Review GitHub PR #123
/review-code --pr 123 --quick             # Quick PR review (Tier 1 only)
/review-code --quick                      # Tier 1 agents only (no multi-LLM)
/review-code --security                   # Security-focused review
/review-code --full                       # All tiers + multi-LLM (most thorough)
/review-code --agents-only                # Skip multi-LLM review
/review-code --llm-only                   # Skip agents, multi-LLM only
```

## Multi-LLM Models (Code Review)

See `.claude/docs/multi-llm-review.md` for model selection, CLI commands, quota limits, and fallback logic. All three tools (Codex, Gemini, seq-server) MUST be used.

## What It Does

```
/review-code [--pr <number>]
         │
         ▼
┌─────────────────────────────────────────┐
│  Step 1: IDENTIFY CHANGES               │
│  - Default: git diff master...HEAD +    │
│    working tree (branch since last PR)  │
│  - On master: falls back to working     │
│    tree (uncommitted) only              │
│  - --uncommitted: git diff (working     │
│    tree only)                           │
│  - --pr: gh pr diff <number>            │
│  - Detect languages (Go, TS, etc.)      │
│  - Identify domains (API, store, UI)    │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  Step 2: RUN CODE AGENTS (Claude)       │
│                                         │
│  ALL READ-ONLY. Never dispatch an       │
│  agent declaring Write/Edit/All tools.  │
│                                         │
│  Tier 1 (always):                       │
│  - race-condition-finder                │
│  - simplicity-reviewer                  │
│  - error-handling-reviewer              │
│                                         │
│  Tier 2 (security):                     │
│  - xss-reviewer                         │
│  - validation-reviewer                  │
│  - permission-auditor                   │
│                                         │
│  Tier 3+ (domain-specific):             │
│  - Based on files changed               │
│                                         │
│  Then: git status --porcelain must be   │
│  unchanged, or an agent wrote to the    │
│  repo. Revert it and say so.            │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  Step 3: MULTI-LLM REVIEW (in parallel) │
│                                         │
│  All models from multi-llm-review.md    │
│  (Codex + Gemini + seq-server)          │
│                                         │
│  Focus: Code quality, edge cases,       │
│  security, performance                  │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  Step 4: SYNTHESIZE FINDINGS            │
│  - Merge agent + LLM findings           │
│  - Prioritize by severity               │
│  - Apply 80/20 filter                   │
│  - Only 2+ model agreement = MUST FIX   │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  OUTPUT: Review Report                  │
│  - MUST FIX (blockers)                  │
│  - SHOULD FIX (quality)                 │
│  - Passed checks                        │
└─────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────┐
│  Step 5: OFFER TO FIX                   │
│  Ask user via AskUserQuestion:          │
│  - "Fix all" (MUST FIX + SHOULD FIX)   │
│  - "MUST FIX only"                      │
│  - "No, just the review"               │
│  Then apply chosen fixes to the code.   │
└─────────────────────────────────────────┘
```

## Multi-LLM Review Commands

Run **all models from `.claude/docs/multi-llm-review.md`** in parallel (single message, multiple tool calls). This includes Codex, Gemini, AND seq-server — do NOT skip any.

## Code Review Prompt Template

```
Review this code for bugs, security issues, and quality.

## Code Changes
<code>
[paste git diff or file contents]
</code>

## CRITICAL: Apply 80/20 Thinking

**A MUST FIX blocker is ONLY:**
- Bug that will cause runtime failure
- Security vulnerability (injection, auth bypass, XSS)
- Data integrity risk (corruption, loss)
- Race condition / concurrency bug
- Missing error handling that crashes

**NOT a blocker (SHOULD FIX or SKIP):**
- Style issues, naming preferences
- Minor optimizations
- Missing comments/docs
- "Best practices" that don't affect correctness

## Evaluate

1. **Correctness**: Will this code work as intended?
2. **Security**: Any vulnerabilities?
3. **Edge Cases**: Null checks, error handling, boundary conditions?
4. **Performance**: Any obvious inefficiencies?
5. **Patterns**: Does it follow established codebase patterns?
6. **Diagnostics**: For Go handler changes — do user-initiated actions and error paths call `PostDiagnostic`? (See `server/CLAUDE.md` → Diagnostics Channel)

## Output

1. **MUST FIX** (0-3 max): What breaks? File:line? Fix?
2. **SHOULD FIX** (0-5): Quality improvements
3. **VERDICT**: APPROVED / NEEDS WORK
```

## Agents must be read-only

**Never dispatch an agent whose definition declares `Write`, `Edit`, or `All
tools`.** A review reads; it does not edit. Every agent in the tiers below is
listed with the tools its definition actually grants, so this is checkable
before you dispatch rather than after.

This is not hypothetical. On 2026-08-15 a `/review-code` run of the mapping
branch dispatched `go-pro` and `react-pro`, both of which declare
`Write, Read, Edit, Bash`. They left five stray `zz_probe*_test.go` files in
`server/decorators/location/` and its `mapdata` package: scratch probes written
into the repo rather than findings reported back.

The debris was the smaller half of the lesson. A second session was editing the
same working tree at the time, and its work was **misread as more agent output
and reverted three times before anyone noticed**. That is the real argument for
this rule: once a review can write, its output is indistinguishable from
everybody else's, and a dirty tree stops being evidence of anything. A reviewer
that cannot write keeps `git status` meaningful.

Two corollaries, both learned the hard way:

- **Stopping an agent does not prove it has stopped writing.** Check the tree,
  do not assume.
- **A dirty tree is not proof an agent did it.** Before reverting anything,
  establish what wrote it. `git status` alone cannot tell you, and reverting is
  not undoable for work that was never staged.

`.claude/docs/multi-llm-review.md` records the same class of failure for Gemini
and fixes it with an isolated working directory. Containment belongs in the
command, not in the prompt wording.

### Before dispatching

Run the check, from the repo root:

```bash
./.claude/skills/review-code/check-agents.sh
```

It reads every agent named in the tier tables below, finds its definition, and
fails if any declares `Write`, `Edit`, `All tools` or `Task`. That covers both
ways this list rots: an agent added to a tier without checking it, and an agent
already in a tier that later gains a tool.

If you dispatch something not in the tables, check it by hand:

1. Read the agent's `tools:` line in `.claude/agents/**/<name>.md`.
2. If it declares `Write`, `Edit`, or `All tools`, **do not use it for review**.
   The banned list below is not exhaustive; the `tools:` line is the authority.
3. `Bash` is permitted but is not read-only either (`sed -i`, `>` redirection).
   Agents that carry it are marked, and the working-tree check below is what
   actually contains them.
4. `Task` is worse than `Bash`: an agent that can spawn agents is not bound by
   this list, since what it spawns is chosen by the model. Agents carrying
   `Task` are excluded.

### After the agent phase, before writing the report

```bash
git status --porcelain
```

The tree must be exactly as it was when the review started. If it is not, an
agent wrote to the repo: revert it (`git checkout -- .` plus removing untracked
files it created), say so in the report, and name the agent so this list can be
corrected. Do not fold an agent's edits into the review as though they were
findings.

Prompts should also say so plainly: end every review prompt with *"Report
findings only. Do not edit any file."* That is advisory, not enforcement, which
is why the check above exists.

### When a write-capable specialist is genuinely the right tool

Dispatch it with `isolation: "worktree"`, which gives it its own git worktree, so
anything it writes lands in a throwaway copy. Note the tradeoff: a worktree is
checked out at `HEAD`, so uncommitted working-tree changes are not visible there
and `--uncommitted` reviews cannot use it.

## Agent Tiers

Tools are from each agent's own definition. `+Bash` means the agent can also
shell out, so it is covered by the working-tree check rather than by its tool
list.

### Tier 1: Core (Always Run)

| Agent | Tools | Catches |
|-------|-------|---------|
| `race-condition-finder` | Read/Grep/Glob +Bash | Concurrency bugs, TOCTOU, data races |
| `simplicity-reviewer` | Read/Grep/Glob | Over-engineering, YAGNI violations |
| `error-handling-reviewer` | Read/Grep/Glob | Missing error checks, swallowed errors |

### Tier 2: Security

| Agent | Tools | Catches |
|-------|-------|---------|
| `xss-reviewer` | Read/Grep/Glob | XSS vulnerabilities in frontend |
| `validation-reviewer` | Read/Grep/Glob | Missing input validation |
| `hardcoded-values-reviewer` | Read/Grep/Glob | Secrets, magic numbers, config in code |
| `permission-auditor` | Read/Grep/Glob | Authorization gaps, permission bypasses |

### Tier 3: Backend (Go files)

| Agent | Tools | Catches |
|-------|-------|---------|
| `concurrent-go-reviewer` | Read/Grep/Glob +Bash | Go concurrency safety |
| `null-safety-reviewer` | Read/Grep/Glob | Nil dereferences in Go and TypeScript |
| `db-call-reviewer` | Read/Grep/Glob +Bash | N+1 queries, unnecessary DB calls |
| `transaction-reviewer` | Read/Grep/Glob | Multi-table operations without transactions |
| `api-reviewer` / `app-reviewer` / `store-reviewer` | Read/Grep/Glob +Bash | Layer-boundary violations |

`go-pro` and `postgres-expert` were here and are banned: both declare `Write`
and `Edit`.

### Tier 4: Frontend (TS/TSX files)

| Agent | Tools | Catches |
|-------|-------|---------|
| `component-reviewer` | Read/Grep/Glob | React component patterns |
| `type-design-analyzer` | Read/Grep/Glob | Type design, encapsulation, invariants |
| `null-safety-reviewer` | Read/Grep/Glob | Null handling in TypeScript |
| `modal-reviewer` | Read/Grep/Glob | Modal patterns |
| `i18n-expert` | Read/Grep/Glob | Untranslated strings, plural forms |

`react-pro` and `typescript-pro` were here and are banned: both declare `Write`
and `Edit`. They are the two that rewrote the repo.

### Tier 5: Testing

| Agent | Tools | Catches |
|-------|-------|---------|
| `test-coverage-reviewer` | Read/Grep/Glob +Bash | Missing test coverage for new code |
| `playwright-patterns-reviewer` | Read/Grep/Glob | E2E test patterns, flaky tests |

`test-unit-expert` (Write/Edit) and `production-validator` (All tools) were here
and are banned. A reviewer that can write tests will write tests.

### Tier 6: Quality/Maintenance (Optional)

| Agent | Tools | When |
|-------|-------|------|
| `duplication-reviewer` | Read/Grep/Glob | Code duplication |
| `comment-analyzer` | Read/Grep/Glob | Comment rot, misleading docs |
| `backwards-compatibility-reviewer` | Read/Grep/Glob | Breaking changes |
| `deprecation-reviewer` | Read/Grep/Glob | Deprecation without a removal path |
| `ha-reviewer` | Read/Grep/Glob | Multi-node correctness |
| `accessibility-guardian` | Read/Grep/Glob +Bash | WCAG, keyboard, screen readers |
| `file-structure-reviewer` | Read/Grep/Glob +Bash | Files that do not match conventions |

### Banned for review

Write-capable, whatever their subject expertise:

| Agent | Declares |
|-------|----------|
| `go-pro`, `react-pro`, `typescript-pro` | `Write, Read, Edit, Bash` |
| `postgres-expert`, `owasp-security`, `threat-modeler` | `Write, Read, Edit, Bash` |
| `test-unit-expert`, `test-e2e-expert`, `websocket-expert` | `Write, Read, Edit, Bash` |
| `tech-debt-surgeon`, `performance-optimizer`, `refactorer`, `code-simplifier` | Write/Edit or All tools |
| `production-validator`, `code-review-swarm`, `pr-manager` | All tools, or Write |
| `pattern-reviewer`, `pr-reviewer`, `migration-code-reviewer` | carry `Task` (or `Edit`), so they delegate outside this list |

If one of these is the only agent that knows a subject, either run it with
`isolation: "worktree"` or ask the question yourself with Read and Grep.

### DO NOT Use for Code Review

These are **PLAN agents** - use them in `/create-plan` and `/review-plan`:
- `design-flaw-finder` - Reviews design, not implementation
- `api-contract-reviewer` - Reviews API design, not handler code
- `database-architecture-reviewer` - Reviews schema design, not queries
- `ux-design-reviewer` - Reviews UX design, not components
- `system-design-reviewer` - Reviews architecture, not code

## Full Agent Reference

For complete agent listing (~140 agents), see `.claude/agents/AGENT_REGISTRY.md`.

## Agent Selection Logic

```python
# Pseudo-logic for agent selection.
#
# Every name here is read-only. Adding one means checking its `tools:` line
# first: an agent that declares Write, Edit, All tools or Task does not belong
# in a review, however well it knows the subject.
agents = []

# Tier 1: Always run (MUST RUN)
agents.extend([
    "race-condition-finder",
    "simplicity-reviewer",
    "error-handling-reviewer",
])

# Tier 2: Security (always for production code)
if not test_files_only:
    agents.extend([
        "hardcoded-values-reviewer",
        "validation-reviewer",
    ])
    if has_ts_files or renders_html:
        agents.append("xss-reviewer")
    if touches_auth:
        agents.append("permission-auditor")

# Tier 3: Backend (Go files)
if has_go_files:
    agents.append("concurrent-go-reviewer")
    agents.append("null-safety-reviewer")
    if has_db_changes:
        agents.extend(["db-call-reviewer", "transaction-reviewer"])

# Tier 4: Frontend (TS/TSX files)
if has_ts_files:
    agents.append("component-reviewer")
    agents.append("type-design-analyzer")

# Tier 5: Testing
if has_test_files:
    agents.append("test-coverage-reviewer")
    if has_e2e_tests:
        agents.append("playwright-patterns-reviewer")
```

## Output Format

```markdown
## Code Review: [files reviewed]

### MUST FIX (Blockers)
| Issue | File:Line | Agent | Fix |
|-------|-----------|-------|-----|
| Race condition in cache access | `cache.go:45` | race-condition-finder | Add mutex |
| Missing permission check | `api.go:123` | permission-auditor | Add HasPermission call |

### SHOULD FIX (Quality)
| Issue | File:Line | Agent | Recommendation |
|-------|-----------|-------|----------------|
| Overly complex function | `utils.go:89` | simplicity-reviewer | Extract helper |

### Passed Checks
- ✅ No XSS vulnerabilities
- ✅ Input validation present
- ✅ Error handling complete
- ✅ Tests cover new code

### Agent Summary
| Agent | Verdict | Findings |
|-------|---------|----------|
| race-condition-finder | ⚠️ ISSUES | 1 race condition |
| simplicity-reviewer | ✅ PASS | - |
| xss-reviewer | ✅ PASS | - |
| permission-auditor | ⚠️ ISSUES | 1 missing check |

---

### Verdict: NEEDS WORK / APPROVED

Fix MUST FIX items before committing.
```

## Offer to Fix

After presenting the review report, **always ask the user if they would like the findings fixed**. Use `AskUserQuestion` to prompt:

> "Would you like me to fix the issues found in the review?"

Options:
- **Fix all** — Apply MUST FIX and SHOULD FIX changes to the code
- **MUST FIX only** — Apply only blocker fixes
- **No, just the review** — Leave the code unchanged

If the user chooses to fix, apply the changes directly to the affected files using `Edit`, preserving existing code structure. After applying fixes, run `make check-style` to ensure formatting is correct.

**Note**: This step is skipped in `--pr` mode since you cannot edit PR code directly. Instead, suggest fixes as PR comments.

## Flags

| Flag | Effect |
|------|--------|
| `--pr <number>` | Review GitHub PR instead of local branch changes |
| `--uncommitted` | Review only uncommitted working-tree changes (old default) |
| `--quick` | Tier 1 agents only, no multi-LLM (fastest) |
| `--security` | Focus on Tier 2 security agents + LLM security review |
| `--full` | All tiers + multi-LLM (most thorough) |
| `--agents-only` | Skip multi-LLM review (Claude agents only) |
| `--llm-only` | Skip agents, run multi-LLM review only |

## Examples

```bash
# Full review of branch changes since last PR (agents + multi-LLM) - RECOMMENDED
/review-code

# Review only uncommitted working-tree changes
/review-code --uncommitted

# Review specific file
/review-code server/app/item_core.go

# Review a GitHub PR
/review-code --pr 123

# Quick PR review (Tier 1 agents only)
/review-code --pr 123 --quick

# Security-focused PR review
/review-code --pr 123 --security

# Quick review (Tier 1 agents only, no LLM)
/review-code --quick

# Security-focused review
/review-code --security

# Full review (all tiers + multi-LLM)
/review-code --full

# Agents only (skip external LLMs)
/review-code --agents-only

# Multi-LLM only (skip agents)
/review-code --llm-only
```

## CLI Reference

See `.claude/docs/multi-llm-review.md` for CLI commands and quota fallback logic.

## When to Use

| Scenario | Command | Skip review |
|----------|---------|-------------|
| After `/create-code` | `/review-code` | |
| Before opening a PR | `/review-code` | |
| Before committing WIP | `/review-code --uncommitted` | |
| Reviewing a PR | `/review-code --pr 123` | |
| Security-sensitive code | `/review-code --security` | |
| Quick PR check | `/review-code --pr 123 --quick` | |
| Tiny typo fix | | ✅ |
| Documentation only | | ✅ |

## Integration with Workflow

```
/create-plan "feature"     # Create plan
    │
    ▼
/create-code plan.md       # Implement
    │
    ▼
/review-code               # Agent review  ← THIS SKILL
    │
    ▼
Fix MUST FIX items
    │
    ▼
/lint                      # Final formatting
    │
    ▼
Commit changes
```

## Tips

- **Run before every commit** - Catch issues early
- **Use `--quick` for WIP** - Full review before PR
- **Fix MUST FIX immediately** - They're blockers for a reason
- **SHOULD FIX can wait** - Address in follow-up if time-constrained
- **Trust multi-model consensus** - 2+ models agreeing = real issue
- **Parallel execution** - Run all LLM calls in single message
- **Use `--agents-only` for speed** - When external LLMs are slow/unavailable
- **Be skeptical of single-model findings** - Could be false positive
