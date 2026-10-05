import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { User } from '../auth/auth.models';

export interface AdminUser extends User { verificationReviewedBy?: number }
@Injectable({ providedIn: 'root' })
export class UserAccessService {
 private readonly http = inject(HttpClient);
 register(input: { fullName: string; email: string; phone: string; password: string }) {
  return this.http.post<{ user: User; message: string }>('/api/auth/university-supervisor-register', input);
 }
 resubmit() { return this.http.post<User>('/api/university-supervisor/verification/resubmit', {}); }
 list(verificationOnly: boolean, filters: Record<string, string>) {
  return this.http.get<User[]>(this.base(verificationOnly), { params: new HttpParams({ fromObject: filters }) });
 }
 detail(id: number, verificationOnly: boolean) { return this.http.get<AdminUser>(`${this.base(verificationOnly)}/${id}`); }
 review(id: number, approve: boolean, reason: string) { return this.http.post<AdminUser>(`${this.base(true)}/${id}/${approve ? 'approve' : 'reject'}`, approve ? {} : { reason }); }
 setActive(id: number, active: boolean) { return this.http.post<AdminUser>(`${this.base(false)}/${id}/${active ? 'enable' : 'disable'}`, {}); }
 private base(verificationOnly: boolean) { return verificationOnly ? '/api/admin/user-verifications' : '/api/admin/users'; }
}
