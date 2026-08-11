import React, {useEffect, useId, useMemo, useRef, useState} from 'react';

import type {ZoneChoice, ZoneGroup} from './zones';

const styles: Record<string, React.CSSProperties> = {
    root: {position: 'relative', marginTop: '8px'},
    input: {
        width: '100%',
        maxWidth: '100%',
        boxSizing: 'border-box',
        padding: '6px 8px',
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        background: 'var(--center-channel-bg)',
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        borderRadius: '4px',
    },
    list: {
        position: 'absolute',
        zIndex: 20,
        top: '100%',
        left: 0,
        right: 0,
        margin: '2px 0 0',
        padding: '4px 0',
        listStyle: 'none',
        maxHeight: '240px',
        overflowY: 'auto',
        background: 'var(--center-channel-bg, #ffffff)',
        border: '1px solid rgba(var(--center-channel-color-rgb), 0.16)',
        borderRadius: '4px',
        boxShadow: '0 6px 16px rgba(0, 0, 0, 0.24)',
    },
    group: {
        padding: '6px 10px 2px',
        fontSize: '10px',
        textTransform: 'uppercase',
        letterSpacing: '0.04em',
        fontWeight: 600,
        opacity: 0.6,
        color: 'var(--center-channel-color)',
    },
    option: {
        padding: '5px 10px',
        fontSize: '13px',
        color: 'var(--center-channel-color)',
        cursor: 'pointer',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
    },
    activeOption: {background: 'rgba(var(--center-channel-color-rgb), 0.08)'},
    offset: {fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', opacity: 0.7},
};

/**
 * Everything about a zone worth typing at it.
 *
 * The identifier appears twice, once as written and once with its separators
 * opened out, so both "america/los" and "los angeles" find Los Angeles. The
 * offset is in there so "+05:30" and "utc-08" work too.
 */
function searchText(zone: ZoneChoice): string {
    return `${zone.name} ${zone.iana} ${zone.iana.replace(/[/_]/g, ' ')} ${zone.offsetLabel}`.
        toLowerCase();
}

/**
 * Every term has to match somewhere, in any order.
 *
 * "raf uk" and "berlin ram" are the queries people actually type, and a single
 * substring would find neither.
 */
function matches(zone: ZoneChoice, terms: string[]): boolean {
    const text = searchText(zone);
    return terms.every((term) => text.includes(term));
}

/**
 * How one zone reads in the list.
 *
 * The name leads when there is one worth leading with, which for this audience
 * is the base rather than the city, and the identifier follows because that is
 * what actually gets stored and two bases can sit behind one of them. Neither
 * is repeated when it would only say the same thing twice.
 */
export function optionLabel(zone: ZoneChoice): string {
    // "Zulu (UTC)" already carries its identifier.
    if (zone.name.includes(zone.iana)) {
        return zone.name;
    }

    // An unnamed zone is called after its own last segment, so the name adds
    // nothing the identifier does not already say.
    const city = zone.iana.split('/').pop()?.replace(/_/g, ' ');
    if (zone.name === zone.iana || zone.name === city) {
        return zone.iana;
    }

    return `${zone.name} (${zone.iana})`;
}

interface Props {

    /** Everything still available to choose, grouped and ordered west to east. */
    groups: ZoneGroup[];

    /** Inert while a save is in flight, or once the table is full. */
    disabled?: boolean;

    onPick: (zone: ZoneChoice) => void;
}

/**
 * Type-to-search picker for a timezone.
 *
 * A combobox rather than a select because there are several hundred zones and
 * every label starts with an offset, which robs a native select of the one way
 * it had of finding anything: its typeahead matches from the start of the
 * option text.
 *
 * The query survives a pick, because adding two bases from one search is the
 * obvious thing to want: the list closes so it does not sit over the buttons
 * below, but the input keeps focus and one arrow key brings the rest back, with
 * whatever was just added gone from it.
 */
const ZonePicker: React.FC<Props> = ({groups, disabled, onPick}) => {
    const [query, setQuery] = useState('');
    const [open, setOpen] = useState(false);
    const [active, setActive] = useState(0);

    const listId = useId();
    const listRef = useRef<HTMLUListElement>(null);

    const shown = useMemo(() => {
        const terms = query.toLowerCase().split(/\s+/).filter(Boolean);
        if (terms.length === 0) {
            return groups;
        }

        return groups.
            map((group) => ({...group, zones: group.zones.filter((zone) => matches(zone, terms))})).
            filter((group) => group.zones.length > 0);
    }, [groups, query]);

    // The options as one sequence, which is what the arrow keys move through:
    // the reader does not have to care that they are grouped.
    const flat = useMemo(() => shown.flatMap((group) => group.zones), [shown]);

    // A new query means a new list, so the old position in it means nothing.
    useEffect(() => {
        setActive(0);
    }, [query]);

    // Whatever is active has to be on screen, or the arrow keys appear to do
    // nothing once the list is longer than its box.
    useEffect(() => {
        if (open) {
            listRef.current?.querySelector('[data-active="true"]')?.scrollIntoView({block: 'nearest'});
        }
    }, [active, open]);

    const pick = (zone: ZoneChoice) => {
        onPick(zone);

        // Closed, or it would sit over the buttons below for as long as the
        // input kept focus. The query survives, and the input still has focus,
        // so adding a second base from the same search is one arrow key away.
        setOpen(false);

        // The list shrinks by one under the cursor, so hold the position rather
        // than letting it drift down it.
        setActive((current) => Math.max(0, Math.min(current, flat.length - 2)));
    };

    const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
        switch (event.key) {
        // Both arrows reopen a closed list without moving, which is what the
        // ARIA combobox pattern asks for. Moving as well would skip the option
        // the reader is being shown for the first time, and it would undo the
        // position pick() deliberately holds: after adding a zone, the one that
        // slid into its place is the likely next choice, not the one after it.
        case 'ArrowDown':
            event.preventDefault();
            if (!open) {
                setOpen(true);
                break;
            }
            setActive((current) => Math.min(current + 1, flat.length - 1));
            break;
        case 'ArrowUp':
            event.preventDefault();
            if (!open) {
                setOpen(true);
                break;
            }
            setActive((current) => Math.max(current - 1, 0));
            break;
        case 'Enter':
            if (open && flat[active]) {
                // Or the surrounding panel would take this as a submission.
                event.preventDefault();
                pick(flat[active]);
            }
            break;
        case 'Escape':
            event.preventDefault();
            setOpen(false);
            break;
        default:
            break;
        }
    };

    let index = -1;

    return (
        <div style={styles.root}>
            <input
                type='text'
                role='combobox'

                // Only expanded when there is a listbox to point at. A query
                // that matches nothing leaves this open with the list unmounted,
                // and announcing an expanded popup that is not in the document
                // leaves a screen reader hunting for something that is not there.
                aria-expanded={open && flat.length > 0}
                aria-controls={listId}
                aria-autocomplete='list'
                aria-activedescendant={open && flat[active] ? `${listId}-${active}` : undefined}
                aria-label='Search timezones'
                autoComplete='off'
                value={query}
                disabled={disabled}
                placeholder='Search timezones and bases...'
                style={styles.input}
                onFocus={() => setOpen(true)}
                onBlur={() => setOpen(false)}
                onKeyDown={onKeyDown}
                onChange={(event) => {
                    setQuery(event.target.value);
                    setOpen(true);
                }}
            />

            {open && flat.length > 0 && (
                <ul
                    id={listId}
                    ref={listRef}
                    role='listbox'
                    aria-label='Timezones'
                    style={styles.list}

                    // Or the input would blur and close the list before the
                    // click ever landed on an option.
                    onMouseDown={(event) => event.preventDefault()}
                >
                    {shown.map((group) => (
                        <React.Fragment key={group.label}>
                            <li
                                role='presentation'
                                data-group={group.label}
                                style={styles.group}
                            >{group.label}</li>

                            {group.zones.map((zone) => {
                                index++;
                                const isActive = index === active;
                                const at = index;

                                return (
                                    <li
                                        key={zone.key}
                                        id={`${listId}-${at}`}
                                        role='option'
                                        aria-selected={isActive}
                                        data-active={isActive ? 'true' : 'false'}
                                        style={isActive ? {...styles.option, ...styles.activeOption} : styles.option}
                                        onMouseEnter={() => setActive(at)}
                                        onClick={() => pick(zone)}
                                    >
                                        <span style={styles.offset}>{`(${zone.offsetLabel}) `}</span>
                                        {optionLabel(zone)}
                                    </li>
                                );
                            })}
                        </React.Fragment>
                    ))}
                </ul>
            )}

            {/*
              * Announced, not just shown. Filtering is the one thing here with
              * no other feedback for somebody who cannot see the list shrink,
              * and "nothing matches" is exactly when they most need telling.
              */}
            {query !== '' && (
                <p
                    role='status'
                    aria-live='polite'
                    style={{fontSize: '12px', opacity: 0.6, margin: '6px 0 0'}}
                >
                    {flat.length === 0 ? 'Nothing matches that.' : `${flat.length} matching.`}
                </p>
            )}
        </div>
    );
};

export default ZonePicker;
