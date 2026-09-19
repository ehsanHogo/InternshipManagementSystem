import { User } from '../auth/auth.models';

export type InternshipCaseStatus = 'DRAFT' | 'UNDER_REVIEW' | 'ACTIVE' | 'COMPLETED';

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
  createdAt: string;
  updatedAt: string;
  submittedAt?: string;
}

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
