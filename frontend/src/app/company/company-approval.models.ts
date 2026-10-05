import { CompanyRating } from '../shared/company-rating.models';
import { Company } from '../internship/internship.models';

export interface EligibleCompany extends Company {
  passedInternshipCount: number;
}

export interface ApprovalCompanyDetail extends EligibleCompany, CompanyRating {
  successfulInternships: {
    caseId: number;
    studentName: string;
    opportunityTitle: string;
    finalResult: 'EXCELLENT' | 'GOOD' | null;
    completedAt: string | null;
  }[];
}

export interface PublicApprovedCompany {
  id: number;
  name: string;
  website: string | null;
  phone: string | null;
  email: string | null;
  address: string | null;
}
