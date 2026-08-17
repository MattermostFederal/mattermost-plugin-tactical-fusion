import React from 'react';

interface Props {
    children: React.ReactNode;
    fallback?: React.ReactNode;
}

interface State {
    failed: boolean;
}

export default class ErrorBoundary extends React.Component<Props, State> {
    public state: State = {failed: false};

    public static getDerivedStateFromError(): State {
        return {failed: true};
    }

    public render() {
        if (this.state.failed) {
            return this.props.fallback ?? null;
        }
        return this.props.children;
    }
}
