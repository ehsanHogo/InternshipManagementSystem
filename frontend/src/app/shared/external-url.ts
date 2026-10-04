/** Ensures bare hostnames open as absolute external links, not relative paths. */
export function externalHref(url: string | null | undefined): string {
  const trimmed = (url ?? '').trim();
  if (!trimmed) {
    return '';
  }
  if (/^https?:\/\//i.test(trimmed)) {
    return trimmed;
  }
  return `https://${trimmed}`;
}
