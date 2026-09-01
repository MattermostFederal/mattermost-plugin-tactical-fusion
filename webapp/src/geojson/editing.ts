import {createEditingStore} from '../decorators/editing';

// eslint-disable-next-line @typescript-eslint/naming-convention
export const {subscribe, isEditing, setEditing, useEditing, _resetForTesting} = createEditingStore();
