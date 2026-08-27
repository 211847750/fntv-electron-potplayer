export function applyVerifiedOriginToFnConnectBaseUrl(
    fnConnectBaseUrl: string,
    verifiedOrigin: string,
): string {
    const base = new URL(fnConnectBaseUrl);
    const pathname = base.pathname === '/' ? '' : base.pathname.replace(/\/$/, '');
    return `${new URL(verifiedOrigin).origin}${pathname}`;
}
