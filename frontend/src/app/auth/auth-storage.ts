const tokenKey = 'internship_access_token';

export function getStoredToken(): string | null {
  return localStorage.getItem(tokenKey);
}

export function storeToken(token: string): void {
  localStorage.setItem(tokenKey, token);
}

export function removeStoredToken(): void {
  localStorage.removeItem(tokenKey);
}
