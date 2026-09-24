export interface CyberRow {
    label: string;
    value: string;
    query: string;
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
    score: string;
    severity: string;
    exploited: boolean;
    vector: CyberVectorMetric[];
    affected: string[];
    configurations: string[];
    references: CyberReference[];
    sections: CyberSection[];
}

export interface CyberSection {
    title: string;
    items: CyberItem[];
}

export interface CyberItem {
    head: string;
    text: string;
    kind: string;
    value: string;
}

export interface CyberVectorMetric {
    metric: string;
    value: string;
    severe: boolean;
}

export interface CyberReference {
    url: string;
    tags: string;
}
