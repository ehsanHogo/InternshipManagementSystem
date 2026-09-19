import { User } from '../auth/auth.models';

export type InternshipCaseStatus =
  | 'DRAFT'
  | 'PENDING_UNIVERSITY_APPROVAL'
  | 'PENDING_COMPANY_APPROVAL'
  | 'COMPANY_APPROVED'
  | 'UNIVERSITY_APPROVED'
  | 'ACTIVE'
  | 'COMPLETED';

export interface Company {
  id: number;
  name: string;
  website?: string;
  phone?: string;
  email?: string;
  address?: string;
  isApproved: boolean;
}

export interface InternshipPreference {
  id: number;
  priority: number;
  companyId?: number;
  company?: Company;
  proposedCompanyName?: string;
  proposedWebsite?: string;
  proposedPhone?: string;
  proposedEmail?: string;
  proposedSupervisorName?: string;
  city: string;
  workField: string;
}

export interface InternshipCase {
  id: number;
  status: InternshipCaseStatus;
  passedCredits: number | null;
  mobile: string | null;
  student: User;
  professor: User;
  preferences: InternshipPreference[];
  selectedPreferenceId?: number;
  selectedPreference?: InternshipPreference;
  companySupervisorId?: number;
  companySupervisor?: User;
  letterNumber?: string;
  letterDate?: string;
  internshipSubject?: string;
  startDate?: string;
  workplaceAddress?: string;
  workplacePhone?: string;
  createdAt: string;
  updatedAt: string;
  submittedAt?: string;
  companyConfirmedAt?: string;
  universityApprovedAt?: string;
  activatedAt?: string;
}

export interface SendToCompanyPayload {
  preferenceId: number;
  companySupervisorId: number;
  letterNumber: string;
  letterDate: string;
}

export interface CompanyConfirmationPayload {
  internshipSubject: string;
  startDate: string;
  workplaceAddress: string;
  workplacePhone: string;
}

export const internshipStatusLabels: Record<InternshipCaseStatus, string> = {
  DRAFT: 'پیش‌نویس',
  PENDING_UNIVERSITY_APPROVAL: 'در انتظار تأیید آموزش',
  PENDING_COMPANY_APPROVAL: 'در انتظار تأیید شرکت',
  COMPANY_APPROVED: 'تأیید شده توسط شرکت',
  UNIVERSITY_APPROVED: 'تأیید شده توسط آموزش',
  ACTIVE: 'کارآموزی فعال',
  COMPLETED: 'تکمیل شده'
};

export interface PreferencePayload {
  priority: number;
  companyId?: number;
  proposedCompanyName?: string;
  proposedWebsite?: string;
  proposedPhone?: string;
  proposedEmail?: string;
  proposedSupervisorName?: string;
  city: string;
  workField: string;
}
