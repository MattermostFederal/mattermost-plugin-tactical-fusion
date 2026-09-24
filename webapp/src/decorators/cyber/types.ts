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
    credits: CyberCredit[];
    glance: CyberGlance;
    reports: CyberThreatReport[];
}

export interface CyberThreatReport {
    source: string;
    malicious: boolean;
    threat: string;
    detail: string;
    url: string;
}

export interface CyberGlance {
    subtitle: string;
    summary: string;
    tags: string[];
    facts: string[];
    status: string;
}

export interface CyberCredit {
    text: string;
    url: string;
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
    url: string;
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

export interface CyberDirectory {
    path: string;
    kind: string;
}

export interface CyberDatasetFile {
    name: string;
    label: string;
    path: string;
    kind: string;
    size: number;
    records: number;
    countError: string;
    generated: string;
}

export interface CyberDatabaseFile {
    path: string;
    kind: string;
    type: string;
    built: string;
    size: number;
}

export interface CyberMissingDataset {
    name: string;
    label: string;
}

export interface CyberSkippedFile {
    path: string;
    reason: string;
}

export interface CyberDatasetsResponse {
    directories: CyberDirectory[];
    datasets: CyberDatasetFile[];
    databases: CyberDatabaseFile[];
    missing: CyberMissingDataset[];
    replaced: string[];
    skipped: CyberSkippedFile[];
}
