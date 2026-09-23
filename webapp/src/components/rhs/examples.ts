import {currentChannel} from '../../decorators/selection';
import {apiBaseUrl} from '../../plugin_url';

export const EXAMPLES_COMMAND = '/tactical-fusion examples';

export async function postExamples(): Promise<void> {
    const channel = currentChannel();
    if (channel === null) {
        throw new Error('Open a channel first.');
    }

    const response = await fetch(`${apiBaseUrl()}/commands/execute`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
            'X-Requested-With': 'XMLHttpRequest',
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({channel_id: channel.channelId, team_id: channel.teamId, command: EXAMPLES_COMMAND}),
    });

    const payload = await response.json().catch(() => null) as {message?: string; text?: string} | null;
    if (!response.ok) {
        throw new Error(payload?.message || `The server returned ${response.status}.`);
    }
    if (payload?.text) {
        throw new Error(payload.text);
    }
}
