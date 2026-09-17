---
name: create-plan
description: Complete plan workflow - researches the codebase, drafts a structured plan, reviews it through parallel read-only review lenses, iterates on blockers, and saves it to implementation-plans/. Outputs a validated plan ready for approval.
user-invocable: true
---

# Create Plan

**Complete plan creation workflow.** Generates a structured plan, reviews it, and iterates until it is ready. Outputs a validated plan ready for user approval.

This skill combines:
1. Codebase research (so the plan follows patterns that already exist)
2. Structured plan generation (a template for consistency)
3. Review through parallel read-only review lenses
4. Iteration on blockers

It needs nothing beyond the built-in tools. If the project also has `/review-plan`, `/create-code` and `/review-code`, they are the natural next steps, but this skill works alone.

## Usage

```
/create-plan <request>                    # Full workflow: research + draft + review + iterate
/create-plan <request> --draft            # Just create the draft, skip review
/create-plan <request> --output <file>    # Save to a specific file
/create-plan <request> --minimal          # Lightweight plan for small features
/create-plan <request> --no-iterate       # Report review findings, do not auto-fix
```

## MANDATORY: Save Plans to implementation-plans/

**YOU MUST WRITE THE PLAN FILE BEFORE EXITING PLAN MODE. NO EXCEPTIONS.**

If you skip this step, the plan is lost and the user cannot reference it later. This is the most common failure mode.

**ALWAYS save the final plan to:**
```
<project-root>/implementation-plans/YY-MM-DD-NN-<feature-name>.md
```

**Naming convention:**
- Prefix with today's date in `YY-MM-DD-` format
- Follow with a zero-padded sequential counter `NN` (01, 02, 03...)
- Before creating a file, glob for `implementation-plans/YY-MM-DD-*` and pick the next number
- Use kebab-case for the feature name after the counter
- Examples: `26-02-15-01-page-tree-reordering.md`, `26-03-01-02-oauth2-support.md`

**CRITICAL WORKFLOW (must follow in order):**

1. **Finalize plan.** Complete all reviews and iterations.
2. **WRITE THE FILE.** Use the `Write` tool to save the complete plan content. Thinking about it or planning to do it is NOT the same as doing it.
3. **VERIFY the file exists.** Use `Read` to confirm the file was written.
4. **Report saved location.** Tell the user: "Plan saved to `implementation-plans/YY-MM-DD-NN-<name>.md`"
5. **Exit plan mode**, if plan mode is active.
6. **User approves.**

**SELF-CHECK before finishing:**
- "Did I call the Write tool to save the plan?" If NO, go back and do it NOW.
- "Can I see the Write tool call in my recent actions?" If NO, the file was NOT written.

## MANDATORY: Implementation Reads from the Saved File

**When implementing a plan, ALWAYS read from the saved file. Never use inline or conversation context.**

After the user approves the plan:

1. **Read the saved plan file**: `Read(implementation-plans/YY-MM-DD-NN-<name>.md)`
2. **Confirm source**: "Implementing from `implementation-plans/YY-MM-DD-NN-<name>.md`"
3. **Follow the plan.** Execute tasks from the file.

**Why this matters:**
- The plan file is the source of truth (version-controlled)
- Conversation context may drift from what was saved
- The user can edit the plan file before implementation
- Multiple sessions can reference the same plan

## What It Does

```
/create-plan "Add OAuth2 support"
         |
         v
  Step 1: RESEARCH
    Read the project's CLAUDE.md and the design notes it points at.
    Explore the codebase for similar features and patterns.
         |
         v
  Step 2: DRAFT PLAN
    Fill in the template.
         |
         v
  Step 3: LENS REVIEW (parallel, read-only)
    Always: design flaws, simplicity.
    By domain: API contract, data and schema, UX and edge states,
    security, type design, client-server alignment.
         |
         v
  Step 4: ITERATE (if needed)
    Fix MUST FIX blockers in the plan. Max 2 iterations.
         |
         v
  Step 5: WRITE FILE (MANDATORY)
    Write to implementation-plans/, Read to verify, report the location.
         |
         v
  Step 6: EXIT PLAN MODE and wait for approval
         |
         v
  Step 7: IMPLEMENTATION (after approval)
    READ from the saved file, not from memory.
```

## Workflow Details

### Step 1: Research the Codebase and Document Patterns

**Read the project's own guidance first.** Its root `CLAUDE.md`, and any design notes, invariants or conventions that file points at for the area being changed. A plan that contradicts a documented invariant is wrong however well it reads.

Then research, and **document the findings in the plan** rather than just holding them:

```
Agent(subagent_type="Explore", prompt="Find how the codebase handles [feature area].
Look for similar features. Return:
1. File:line references for patterns to follow
2. Conventions used
3. Anti-patterns to avoid")
```

**Research output should include:**
- 2-3 similar features in the codebase, with file references
- The patterns they follow (with file:line)
- Why this approach over the alternatives
- Current gaps (what is missing today)

### Step 2: Draft the Plan Using the Template

```markdown
# [Feature Name]

## Overview
[1-2 sentence summary]

## Problem Statement
[What problem does this solve? Why is it needed now?]

## Current State
[How does it work today? What exists already?]

### Current Gaps
- [Gap 1]
- [Gap 2]

## Design Principles
| Pattern | Our Approach | Avoid | Reference |
|---------|-------------|--------|-----------|
| [e.g., Cancellation] | [what we do] | [what we do not] | `[file:line]` |

## Reference Patterns
Similar features to follow:
- `[file:line]` - [what pattern it demonstrates]
- `[file:line]` - [what pattern it demonstrates]

## Requirements
- [ ] Requirement 1
- [ ] Requirement 2

## Out of Scope
- NOT doing X
- NOT doing Y

## Technical Approach
[How we will build it, aligned with the design principles above]

## Decisions
| Question | Decision | Rationale |
|----------|----------|-----------|

## Files to Modify
| File | Change |
|------|--------|

## Tasks
1. [ ] Task 1
2. [ ] Task 2

## Risks & Mitigations
| Risk | Mitigation |
|------|------------|

## UX Summary (for UI features)
| Scenario | Behavior |
|----------|----------|

## Testing Plan
**Unit**: [what]
**Integration**: [what]
**E2E**: [what]

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Project Checklist
- [ ] Every invariant and convention in the project's CLAUDE.md that this touches is named, with how the plan honors it
- [ ] Documentation, help pages and design notes that must change with the code are listed under Files to Modify
```

### Template Section Guidelines

| Section | When to Include |
|---------|-----------------|
| **Problem Statement** | Always |
| **Current State / Gaps** | Always |
| **Design Principles** | Always |
| **Reference Patterns** | When following existing patterns |
| **Decisions** | When making non-obvious choices |
| **UX Summary** | UI features only |
| **Testing Plan** | Features requiring new tests |
| **Project Checklist** | Always |
| **Phase Strategy** | Large features, see below |

With `--minimal`, keep only Overview, Requirements, Out of Scope, Files to Modify, Tasks and Acceptance Criteria.

### Phase Strategy (for large features)

For features spanning multiple phases, add after Problem Statement:

```markdown
## Phase Strategy

| Phase | Focus | Value |
|-------|-------|-------|
| **Phase 1** | Core MVP: [key deliverables] | **80% of value** |
| **Phase 2** | Polish: [edge cases, cleanup] | Robustness |
| **Phase 3** | Enhanced: [nice-to-haves] | Optional |

### Phase 1 Scope (this plan)
[Details for Phase 1 only]

### Deferred to Phase 2+
- [Item] - Phase 2
```

### Step 3: Lens Review

A **lens** is one narrow question asked of the plan by a fresh reviewer that has not seen the drafting conversation. Each lens runs as a built-in read-only `Explore` subagent. Launch every selected lens in parallel, in a single message.

Plans are not code. These lenses review design, sequence and contracts. Lenses that look for implementation bugs belong to code review, after the code exists.

#### Always run

| Lens | Asks |
|------|------|
| Design flaws | Logical flaws, missing steps, impossible sequences, contradictions, states nobody handles |
| Simplicity | Over-engineering, YAGNI violations, premature abstraction, anything that can be cut |

#### Run when the plan touches the domain

| Plan mentions | Lens | Asks |
|---------------|------|------|
| API, endpoint, route, REST, payload | API contract | Completeness, consistency, breaking changes, versioning, auth on every route |
| database, schema, migration, table, index, store | Data and schema | Integrity, migrations and rollback, indexes, unbounded queries |
| UI, component, modal, page, panel | UX and edge states | User flows, and the error, empty and loading states |
| permission, role, auth, token, secret, user input | Security | Trust boundaries, authorization gaps, injection surfaces, what an attacker controls |
| type, struct, interface, model | Type design | Invariants the types express or fail to, encapsulation |
| both a server and a client change | Client-server alignment | Methods, paths, request and response shapes agree on both sides |
| several components or layers | System design | Boundaries, responsibilities in the wrong layer, coupling of independent concerns |

#### Lens prompt

```
You are reviewing an implementation PLAN, not code, through one lens: [lens name].
[The "Asks" text for the lens.]

You may read the repository to check the plan's claims against what exists.

<plan>
[full plan]
</plan>

Apply 80/20 thinking. A MUST FIX is only something that makes the plan
impossible to implement, risks data loss or a security hole, leaves a contract
undefined, or forces the implementer to guess. Everything else is SHOULD FIX,
DEFER or SKIP.

Output: MUST FIX (0-3), SHOULD FIX (0-5), DEFER, SKIP, and a PASS/FAIL verdict.
Report findings only. Do not edit any file.
```

#### Judging the findings

A lens is a fresh reader, not an authority. Before treating a finding as MUST FIX, verify it yourself against the plan and the code. Be skeptical: most findings are SHOULD FIX or DEFER, and a suggestion that adds scope is usually SKIP.

### Step 4: Iterate

If MUST FIX items were found:
1. Update the plan to address the blockers
2. Re-run the affected lenses if the change was major
3. Maximum 2 iterations. If it is still blocked, rethink the approach.

## Output Format

```markdown
## Plan Review Summary

### Status: READY / NEEDS ATTENTION

### Lens Review
| Lens | Verdict | Key Findings |
|------|---------|--------------|
| Design flaws | PASS/FAIL | [summary] |
| Simplicity | PASS/FAIL | [summary] |
| [domain lenses] | PASS/FAIL | [summary] |

### Resolved Blockers
- [What was fixed]

### Remaining SHOULD FIX
- [Item] - [recommendation]

---

[Full plan content]
```

## Flags

| Flag | Effect |
|------|--------|
| `--draft` | Skip the review, just generate the plan |
| `--minimal` | Abbreviated template for small features |
| `--output <path>` | Save to a specific file |
| `--no-iterate` | Do not auto-fix, just report findings |

## Examples

```bash
# Full workflow
/create-plan "Add OAuth2 support with Google and GitHub providers"

# Just draft, no review
/create-plan "Add loading spinner" --draft

# Minimal plan for a small feature
/create-plan "Fix pagination bug" --minimal
```

## When to Use

| Scenario | Use `/create-plan` | Just ask |
|----------|--------------------|----------|
| New feature | Yes | |
| Multi-file change | Yes | |
| API or data changes | Yes | |
| UI/UX changes | Yes | |
| Simple bug fix | | Yes |
| Single-file tweak | | Yes |

## Tips

- **The two always-run lenses are cheap.** Do not skip them.
- **Pick domain lenses from the plan's content**, not from habit.
- **Verify before believing.** A lens finding is a lead, not a verdict.
- **Max 2 iterations.** If still blocked, rethink the approach.
