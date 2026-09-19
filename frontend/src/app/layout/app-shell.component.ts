import { Component, inject } from '@angular/core';
import { Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';

import { UserRole } from '../auth/auth.models';
import { AuthService } from '../auth/auth.service';

const roleLabels: Record<UserRole, string> = {
  STUDENT: 'دانشجو',
  PROFESSOR: 'استاد',
  COMPANY_SUPERVISOR: 'سرپرست شرکت',
  UNIVERSITY_SUPERVISOR: 'مسئول دانشگاه',
  ADMIN: 'مدیر سیستم'
};

@Component({
  selector: 'app-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, ButtonModule, TagModule],
  templateUrl: './app-shell.component.html',
  styleUrl: './app-shell.component.scss'
})
export class AppShellComponent {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  readonly user = this.auth.user;

  roleLabel(role: UserRole): string {
    return roleLabels[role];
  }

  logout(): void {
    this.auth.logout();
    void this.router.navigateByUrl('/login');
  }
}
