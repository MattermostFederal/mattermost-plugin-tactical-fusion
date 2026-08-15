#!/usr/bin/env bash
#
# Every agent this skill dispatches must be read-only.
#
# The tier tables name agents; the agent definitions declare what those agents
# can do. Nothing keeps the two in step, and the failure is silent: a reviewer
# that gains Write starts editing the repo instead of reporting on it, which is
# what go-pro and react-pro did on 2026-08-15.
#
# Run from the repo root. Exits non-zero and names the offenders.
set -u

skill="$(dirname "$0")/SKILL.md"
[ -f "$skill" ] || { echo "no SKILL.md beside $0"; exit 2; }

# The agents the skill tells you to dispatch: the tier tables, which end where
# the banned list begins.
names=$(awk '/^## Agent Tiers/{on=1} /^### Banned for review/{on=0} on' "$skill" |
    grep -o '^| `[a-z0-9-]*`' | tr -d '|` ' | sort -u)

[ -n "$names" ] || { echo "no agents found in the tier tables; has SKILL.md changed shape?"; exit 2; }

fail=0
for name in $names; do
    def=$(find .claude/agents "$HOME/.claude/agents" -name "$name.md" 2>/dev/null | head -1)
    if [ -z "$def" ]; then
        echo "MISSING  $name: named by the skill but no definition found"
        fail=1
        continue
    fi

    tools=$(grep -m1 -i '^tools:' "$def" | sed 's/^[Tt]ools:[[:space:]]*//')
    case "$tools" in
        *Write*|*Edit*|*"All tools"*|*Task*)
            echo "WRITABLE $name: declares '$tools'"
            echo "         ($def)"
            fail=1
            ;;
    esac
done

if [ "$fail" -ne 0 ]; then
    echo
    echo "A review reads; it does not edit. Move these to the banned list, or"
    echo "replace them with a read-only agent that covers the same ground."
    exit 1
fi

echo "all $(echo "$names" | wc -w | tr -d ' ') agents named by this skill are read-only"
