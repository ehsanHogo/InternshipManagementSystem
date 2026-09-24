export type OpportunityStatus = 'OPEN' | 'CLOSED';

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

export interface StudentOpportunity extends OpportunityPayload {
  id: number;
  createdAt: string;
  company: OpportunityCompany;
}

export const opportunityStatusLabels: Record<OpportunityStatus, string> = {
  OPEN: 'باز',
  CLOSED: 'بسته'
};
