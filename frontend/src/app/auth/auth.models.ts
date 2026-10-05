export type UserRole =
  | 'STUDENT'
  | 'PROFESSOR'
  | 'COMPANY_SUPERVISOR'
  | 'UNIVERSITY_SUPERVISOR'
  | 'ADMIN';

export type UserVerificationStatus = 'NOT_REQUIRED' | 'PENDING' | 'APPROVED' | 'REJECTED';
export const verificationLabels: Record<UserVerificationStatus, string> = {
 NOT_REQUIRED: 'نیاز ندارد', PENDING: 'در انتظار بررسی مدیر سیستم', APPROVED: 'تأیید شده', REJECTED: 'رد شده'
};
export const roleLabels: Record<UserRole, string> = {
 STUDENT: 'دانشجو', PROFESSOR: 'استاد', COMPANY_SUPERVISOR: 'سرپرست شرکت', UNIVERSITY_SUPERVISOR: 'مسئول آموزش', ADMIN: 'مدیر سامانه'
};
export interface User {
  isActive: boolean;
  createdAt: string;
  verificationStatus: UserVerificationStatus;
  verificationReviewedAt?: string;
  verificationRejectionReason?: string;
  verificationResubmittedAt?: string;
  id: number;
  fullName: string;
  email: string;
  role: UserRole;
  studentNumber?: string;
  major?: string;
  phone?: string;
  jobTitle?: string;
  companyId?: number;
  companyName?: string;
  companyRegistrationStatus?: import('../internship/internship.models').CompanyRegistrationStatus;
}

export interface LoginResponse {
  token: string;
  user: User;
}
