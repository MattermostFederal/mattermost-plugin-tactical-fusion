declare module '*.mjs' {
    const url: string;
    export default url;
}

declare module '*.css';

declare module '*.mjs?copy' {
    const url: string;
    export default url;
}
