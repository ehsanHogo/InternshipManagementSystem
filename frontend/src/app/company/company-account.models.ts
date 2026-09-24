import { User } from '../auth/auth.models';
import { Company } from '../internship/internship.models';

export interface CompanyRegistrationPayload {
  supervisor: {
    fullName: string;
    email: string;
    password: string;
    phone: string;
    jobTitle: string;
  };
  company: {
    name: string;
    nationalId: string;
    economicCode: string;
    website: string;
    phone: string;
    email: string;
    address: string;
  };
}

export interface CompanyAccountProfile {
  company: Company;
  supervisor: User;
}

export interface CompanyRegistrationResponse {
  message: string;
  account: CompanyAccountProfile;
}
