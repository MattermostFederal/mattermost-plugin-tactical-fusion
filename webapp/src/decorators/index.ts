import dtg from './dtg';
import location from './location';
import {get, register} from './registry';

/**
 * Registers the decorators this bundle ships.
 *
 * Adding a decorator is one line here plus one directory. Nothing else in
 * `decorators/` changes.
 *
 * Idempotent, because the registry lives in module state that survives a plugin
 * re-registration while `initialize()` runs again. Throwing on the second pass
 * would leave the sidebar dead until a page reload.
 */
export function registerBuiltinDecorators(): void {
    for (const decorator of [dtg, location]) {
        if (!get(decorator.type)) {
            register(decorator);
        }
    }
}
