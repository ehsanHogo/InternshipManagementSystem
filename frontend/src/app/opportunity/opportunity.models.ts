import type { StatusTagSeverity } from '../internship/internship.models';

export type OpportunityStatus = 'OPEN' | 'CLOSED';
export type ApplicationStatus = 'PENDING' | 'ACCEPTED' | 'REJECTED';

export interface OpportunityPayload {
  title: string;
  description: string;
  workField: string;
  location: string;
}

export interface CompanyOpportunity extends OpportunityPayload {
  id: number;
  status: OpportunityStatus;
  createdAt: string;
  updatedAt: string;
}

export interface OpportunityCompany {
  id: number;
  name: string;
  website?: string;
  phone?: string;
  email?: string;
  address?: string;
  isApproved: boolean;
}

export interface ExistingOpportunityApplication {
  id: number;
  status: ApplicationStatus;
}

export interface StudentOpportunity extends OpportunityPayload {
  id: number;
  createdAt: string;
  company: OpportunityCompany;
  canApply: boolean;
  applyRestrictionCode?: string;
  existingApplication?: ExistingOpportunityApplication;
}

export function opportunityApplyRestrictionMessage(code?: string): string {
  switch (code) {
    case 'INTERNSHIP_ALREADY_COMPLETED':
      return 'شما قبلاً دوره کارآموزی خود را با موفقیت گذرانده‌اید و امکان ثبت درخواست جدید ندارید.';
    case 'INTERNSHIP_CASE_ALREADY_IN_PROGRESS':
      return 'پرونده کارآموزی شما در حال بررسی یا اجرا است و در حال حاضر امکان ارسال درخواست جدید وجود ندارد.';
    case 'OPPORTUNITY_NOT_OPEN':
      return 'این فرصت کارآموزی بسته شده است و درخواست جدید نمی‌پذیرد.';
    case 'APPLICATION_ALREADY_EXISTS':
      return 'درخواست شما برای این فرصت قبلاً ثبت شده است.';
    default:
      return 'در حال حاضر امکان ارسال درخواست برای این فرصت وجود ندارد.';
  }
}

export interface ApplicationStudent {
  id: number;
  fullName: string;
  email: string;
}

export interface ApplicationResume {
  id: number;
  originalName: string;
  uploadedAt: string;
}

export interface OpportunityApplication {
  id: number;
  status: ApplicationStatus;
  companyComment?: string;
  appliedAt: string;
  reviewedAt?: string;
  opportunity: {
    id: number;
    title: string;
    workField: string;
    location: string;
    company: OpportunityCompany;
  };
  student?: ApplicationStudent;
  resume: ApplicationResume;
}

export const opportunityStatusLabels: Record<OpportunityStatus, string> = {
  OPEN: 'باز',
  CLOSED: 'بسته'
};

export const applicationStatusLabels: Record<ApplicationStatus, string> = {
  PENDING: 'در انتظار بررسی',
  ACCEPTED: 'پذیرفته شده',
  REJECTED: 'رد شده'
};

export function opportunityStatusSeverity(status: OpportunityStatus): StatusTagSeverity {
  return status === 'OPEN' ? 'success' : 'secondary';
}

export function applicationStatusSeverity(status: ApplicationStatus): StatusTagSeverity {
  switch (status) {
    case 'ACCEPTED':
      return 'success';
    case 'REJECTED':
      return 'danger';
    default:
      return 'warn';
  }
}
