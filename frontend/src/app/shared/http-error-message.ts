import { HttpErrorResponse } from "@angular/common/http";

const TECHNICAL_ERROR =
  /(record not found|duplicate key|pq:|gorm:|sql|unauthorized|forbidden|invalid status|internal server error|stack trace)/i;
const PERSIAN_TEXT = /[\u0600-\u06ff]/;

export function userErrorMessage(
  error: HttpErrorResponse,
  fallback: string,
): string {
  if (error.status === 0) return "ارتباط با سرور برقرار نشد.";

  const message =
    typeof error.error?.error === "string" ? error.error.error.trim() : "";
  if (message && PERSIAN_TEXT.test(message) && !TECHNICAL_ERROR.test(message))
    return message;

  return fallback;
}
