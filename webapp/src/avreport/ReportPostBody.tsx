import React from 'react';

import ReportCard from './ReportCard';
import {fromProps} from './types';

interface PostLike {
    id?: string;
    message?: string;
    props?: unknown;
    edit_at?: number;
}

interface Props {
    post?: PostLike;
    compactDisplay?: boolean;
}

const styles: Record<string, React.CSSProperties> = {
    message: {whiteSpace: 'pre-wrap'},
};

export const ReportPostBody: React.FC<Props> = ({post, compactDisplay}) => {
    const message = post?.message ?? '';
    const plain = <span style={styles.message}>{message}</span>;

    const payload = fromProps(post?.props);
    if (payload === null || (post?.edit_at ?? 0) !== 0) {
        return plain;
    }

    return (
        <ReportCard
            payload={{...payload, postId: post?.id ?? ''}}
            compactDisplay={compactDisplay}
        />
    );
};

export default ReportPostBody;
