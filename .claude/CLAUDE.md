# Project Guidelines

## Overview
<!-- Describe your project here -->

## Architecture
<!-- Document your project's architecture and key directories -->

## Coding Conventions
- **Do not comment the code.** No prose comments in new or modified code, and
  delete the ones in code you touch. This overrides "match the style of
  surrounding code" below, because much of this repository predates the rule.
  Keep directives (`//go:embed`, `//go:generate`, `//nolint:...`, `//go:build`,
  `// #nosec ...`, `// eslint-disable-*`) and generated-file and license headers.
- Durable rationale goes in the root `CLAUDE.md`, not inline. Move a comment's
  content there before deleting it. See its "Coding conventions" section.
- Follow existing patterns in the codebase
- Match the style of surrounding code, except for comments
- Use meaningful variable and function names, so the intent reads without a
  comment explaining it
- Keep functions focused and small

## Testing
- Write tests for new functionality
- Run existing tests before submitting changes
- Aim for meaningful coverage of critical paths

## Git Workflow
- Create feature branches for new work
- Write clear, descriptive commit messages
- Keep commits focused on a single change

## Error Handling
- Handle errors explicitly, don't ignore them
- Provide context when wrapping errors
- Log errors at appropriate levels

## Dependencies
- Prefer standard library solutions when available
- Evaluate dependencies for maintenance and security
- Keep dependencies up to date
