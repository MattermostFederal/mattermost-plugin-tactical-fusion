import {createEditingStore} from '../decorators/editing';

/*
 * Whether the Cursor on Target panel is showing its editor.
 *
 * Its own store, so opening this editor does not open a decorator's.
 * See createEditingStore for why that is a factory rather than three files.
 */
// eslint-disable-next-line @typescript-eslint/naming-convention
export const {subscribe, isEditing, setEditing, useEditing, _resetForTesting} = createEditingStore();
