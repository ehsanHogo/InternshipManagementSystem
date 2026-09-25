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
