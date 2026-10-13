export function adminHeaders(sessionToken: string, extra: Record<string, string> = {}): Record<string, string> {
  return sessionToken ? { ...extra, Authorization: `Bearer ${sessionToken}` } : { ...extra };
}
