export interface CyberRow {
    label: string;
    value: string;
}

export interface CyberLink {
    kind: string;
    value: string;
    label: string;
}

export interface CyberWatchEntry {
    verdict: string;
    source: string;
    note: string;
    updated: string;
    known: boolean;
}

export interface CyberDataset {
    name: string;
    label: string;
    present: boolean;
    generated: string;
}

export interface CyberResponse {
    kind: string;
    value: string;
    title: string;
    headline: string;
    summary: string;
    status: string;
    rows: CyberRow[];
    related: CyberLink[];
    watchlist: CyberWatchEntry[];
    datasets: CyberDataset[];
    affected: string[];
    configurations: string[];
    references: CyberReference[];
}

export interface CyberReference {
    url: string;
    tags: string;
}

export interface CyberMention {
    postId: string;
    channelId: string;
    channel: string;
    createAt: number;
    snippet: string;
    permalink: string;
}

export interface CyberMentionsResponse {
    value: string;
    mentions: CyberMention[];
    truncated: boolean;
}
