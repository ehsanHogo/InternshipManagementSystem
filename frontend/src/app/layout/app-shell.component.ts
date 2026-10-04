import { Component, inject, signal } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { ButtonModule } from 'primeng/button';
import { Drawer } from 'primeng/drawer';
import { TagModule } from 'primeng/tag';
import { filter } from 'rxjs';

import { UserRole } from '../auth/auth.models';
import { AuthService } from '../auth/auth.service';

const roleLabels: Record<UserRole, string> = {
  STUDENT: 'دانشجو',
  PROFESSOR: 'استاد',
  COMPANY_SUPERVISOR: 'سرپرست شرکت',
  UNIVERSITY_SUPERVISOR: 'مسئول آموزش',
  ADMIN: 'مدیر سامانه'
};

@Component({
  selector: 'app-shell',
  imports: [RouterOutlet, RouterLink, RouterLinkActive, ButtonModule, Drawer, TagModule],
  templateUrl: './app-shell.component.html',
  styleUrl: './app-shell.component.scss'
})
export class AppShellComponent {
  private readonly auth = inject(AuthService);
  private readonly router = inject(Router);

  readonly user = this.auth.user;
  readonly navOpen = signal(false);

  constructor() {
    this.router.events
      .pipe(
        filter((event): event is NavigationEnd => event instanceof NavigationEnd),
        takeUntilDestroyed()
      )
      .subscribe(() => this.navOpen.set(false));
  }

  roleLabel(role: UserRole): string {
    return roleLabels[role];
  }

  openNav(): void {
    this.navOpen.set(true);
  }

  onNavVisibleChange(visible: boolean): void {
    this.navOpen.set(visible);
  }

  logout(): void {
    this.auth.logout();
    void this.router.navigateByUrl('/login');
  }
}
