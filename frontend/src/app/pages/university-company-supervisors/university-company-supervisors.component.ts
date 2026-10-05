import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { MessageService } from 'primeng/api';
import { ButtonModule } from 'primeng/button';
import { TableModule } from 'primeng/table';

import { CompanySupervisor } from '../../university-management/university-management.models';
import { UniversityManagementService } from '../../university-management/university-management.service';
import { userErrorMessage } from '../../shared/http-error-message';

@Component({
  selector: 'app-university-company-supervisors',
  imports: [ButtonModule, TableModule],
  templateUrl: './university-company-supervisors.component.html',
  styleUrl: '../workflow-page.scss'
})
export class UniversityCompanySupervisorsComponent {
  private readonly management = inject(UniversityManagementService);
  private readonly messages = inject(MessageService);

  readonly supervisors = signal<CompanySupervisor[]>([]);
  readonly loading = signal(true);
  readonly loadFailed = signal(false);
  constructor() { this.load(); }

  load(): void {
    this.loading.set(true);
    this.loadFailed.set(false);
    this.management.listCompanySupervisors().subscribe({
      next: (supervisors) => { this.supervisors.set(supervisors); this.loading.set(false); },
      error: (error: HttpErrorResponse) => {
        this.loading.set(false);
        this.loadFailed.set(true);
        this.showError(error, 'دریافت اطلاعات ناموفق بود.');
      }
    });
  }

  private showError(error: HttpErrorResponse, fallback: string): void {
    this.messages.add({ severity: 'error', summary: 'خطا', detail: userErrorMessage(error, fallback) });
  }
}
