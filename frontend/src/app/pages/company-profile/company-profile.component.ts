import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';

import { CompanyAccountProfile } from '../../company/company-account.models';
import { CompanyAccountService } from '../../company/company-account.service';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
  selector: 'app-company-profile',
  imports: [ButtonModule, CardModule, TagModule],
  templateUrl: './company-profile.component.html',
  styleUrl: './company-profile.component.scss'
})
export class CompanyProfileComponent {
  private readonly companyAccount = inject(CompanyAccountService);
  private readonly messages = inject(MessageService);

  readonly profile = signal<CompanyAccountProfile | null>(null);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);

  constructor() {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.companyAccount.getProfile().subscribe({
      next: (profile) => {
        this.profile.set(profile);
        this.loading.set(false);
      },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.messages.add({
          severity: 'error',
          summary: 'خطا',
          detail: userErrorMessage(error, 'دریافت اطلاعات شرکت ناموفق بود.')
        });
      }
    });
  }
}
