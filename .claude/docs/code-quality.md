# Code Quality Standards

## Code Comments Guidelines

See `~/CLAUDE.md` -> "Code Comments" for the full rule. In short: **do not write
prose comments, and remove the ones you encounter in code you touch.** Compiler
and tool directives, generated-file headers, and license headers stay, because
they are syntax rather than prose.

This section previously listed the kinds of comment never to add. That taxonomy
is gone because none of it is needed once no prose comment is added at all.

## Code Movement/Refactoring
When moving or reorganizing code:
- **ALWAYS COPY, NEVER RECREATE**: Copy exact implementation, don't retype
- **Use Read tool first**: Get exact implementation before moving
- **Strip comments on the way**: Do not carry prose comments into the new
  location. Keep directives and generated-file headers
- **No provenance comments**: Don't add "COPIED from", "Moved from"
- **Verify after move**: Check both old and new locations

## Implementation Rules
- **NEVER optimize for "getting tests passing quickly"**: Fix actual issues
- **NEVER hardcode values**: Always define constants
- **NEVER add stubs/placeholders/TODOs**: Implement complete solutions
- **NO OVERENGINEERING**: Simplest solution that works, follow YAGNI
- **Align with existing patterns**: Copy structure and style of similar code

## Quality Checks
- Make surgical changes only - preserve existing patterns
- Never create commits without explicit instruction
- Follow existing code style exactly
- Fix tests to match correct behavior
- Add explicit TypeScript types, fix all linter errors
- Mock only what you use, test only what you call
- **NEVER write empty or skipped tests**
- **ENSURE existing tests pass** after implementation
