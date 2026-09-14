import type React from 'react';

export const BRIDGE_API_VERSION = 1;

export const GLOBAL_NAME = 'TacticalFusion';

export const READY_EVENT = 'tactical-fusion:ready';

export type DeclineReason = 'unknown_type' | 'not_recognized' | 'disabled';

export interface DecorateRequest {
    message: string;
    reference_time?: number;
}

export interface DecorateResponse {
    message: string;
    changed: boolean;
    fits_post: boolean;
}

export interface LinkRequest {
    type: string;
    token: string;
    label?: string;
    reference_time?: number;
}

export interface LinkResponse {
    markdown: string;
    url: string;
    type: string;
    label: string;
}

export interface ErrorResponse {
    message: string;
    code: number;
    reason?: string;
}

export type ReferenceTime = Date | number;

export interface DecorateOptions {
    referenceTime?: ReferenceTime;
    signal?: AbortSignal;
}

export interface LinkOptions extends DecorateOptions {
    label?: string;
}

export interface DecoratedText {
    message: string;
    changed: boolean;
    fitsPost: boolean;
}

export interface DecoratorLink {
    markdown: string;
    url: string;
    type: string;
    label: string;
}

export interface LinkProps {
    type: string;
    token: string;
    label?: string;
    referenceTime?: ReferenceTime;
    fallback?: React.ReactNode;
}

export interface TacticalFusionApi {
    readonly apiVersion: number;
    readonly version: string;
    readonly types: readonly string[];
    decorate(message: string, options?: DecorateOptions): Promise<DecoratedText>;
    link(type: string, token: string, options?: LinkOptions): Promise<DecoratorLink>;
    readonly Link: React.ComponentType<LinkProps>;
}

declare global {
    interface Window {
        TacticalFusion?: TacticalFusionApi;
    }
}
