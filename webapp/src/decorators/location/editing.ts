import {createEditingStore} from '../editing';

/*
 * Whether the location panel is showing its editor.
 *
 * Its own store, so opening this editor does not open the other decorator's.
 * See createEditingStore for why that is a factory rather than two files.
 */
// eslint-disable-next-line @typescript-eslint/naming-convention
export const {subscribe, isEditing, setEditing, useEditing, _resetForTesting} = createEditingStore();
