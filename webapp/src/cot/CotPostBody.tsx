import React from 'react';

import CotCard, {fileHref} from './CotCard';
import {SOURCE_FILE, fromProps} from './types';

interface PostLike {
    id?: string;
    message?: string;
    props?: unknown;
    edit_at?: number;
    create_at?: number;
    file_ids?: string[];
}

interface Props {
    post?: PostLike;
    compactDisplay?: boolean;
}

const styles: Record<string, React.CSSProperties> = {
    message: {whiteSpace: 'pre-wrap'},
    files: {opacity: 0.75},
};

/**
 * The fallback, which may never render nothing.
 *
 * A file-case post has an empty message by construction, and a post whose type
 * this plugin owns has already lost its embeds, though NOT its files, so a
 * props loss would otherwise leave a permanently blank post with no way out.
 */
function Fallback({post}: {post?: PostLike}) {
    const message = post?.message ?? '';
    if (message !== '') {
        return <span style={styles.message}>{message}</span>;
    }

    const fileIds = post?.file_ids ?? [];
    if (fileIds.length === 0) {
        return <span style={styles.message}>{'This post carries no readable content.'}</span>;
    }

    return (
        <span style={styles.files}>
            {'Attached: '}
            {fileIds.map((id, index) => {
                const href = fileHref(id);
                const name = `file ${index + 1} of ${fileIds.length}`;
                return (
                    <React.Fragment key={id}>
                        {index > 0 && ', '}
                        {href === null ? name : <a href={href}>{name}</a>}
                    </React.Fragment>
                );
            })}
        </span>
    );
}

export const CotPostBody: React.FC<Props> = ({post, compactDisplay}) => {
    const payload = fromProps(post?.props);
    if (payload === null) {
        return <Fallback post={post}/>;
    }

    // An edit is the one event that can make the props and the message
    // disagree. Post.Type survives it and Props may not, so the card stands
    // down rather than describing something that is no longer there.
    if ((post?.edit_at ?? 0) !== 0) {
        return <Fallback post={post}/>;
    }

    if (payload.source === SOURCE_FILE && !(post?.file_ids ?? []).includes(payload.fileId)) {
        return <Fallback post={post}/>;
    }

    return (
        <CotCard
            payload={{...payload, postId: post?.id ?? ''}}
            compactDisplay={compactDisplay}
        />
    );
};

export default CotPostBody;
