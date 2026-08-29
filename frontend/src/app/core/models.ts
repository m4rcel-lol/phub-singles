/** Shapes returned by the Go API. Kept in one file so the contract is easy to
 *  audit against backend/internal/httpx. */

export interface Profile {
  /** Page handle: the public profile lives at /<username>. */
  username: string;
  displayName: string;
  tagline: string;
  bio: string;
  avatarUrl: string;
  updatedAt: string;
}

/** Link as exposed to the public page: no counters, no disabled entries. */
export interface PublicLink {
  id: number;
  title: string;
  url: string;
  icon: string;
}

/** Link as exposed to the admin panel. */
export interface AdminLink extends PublicLink {
  enabled: boolean;
  position: number;
  clicks: number;
  createdAt: string;
  updatedAt: string;
}

/** Owner-controlled copy shown on the landing page. */
export interface SiteInfo {
  headline: string;
  lede: string;
}

export interface PagePayload {
  site: SiteInfo;
  profile: Profile;
  badges: Badge[];
  links: PublicLink[];
}

export interface LinkPayload {
  title: string;
  url: string;
  icon: string;
  enabled: boolean;
}

export interface LinkStat {
  id: number;
  title: string;
  icon: string;
  url: string;
  enabled: boolean;
  clicks: number;
}

export interface DayStat {
  day: string;
  views: number;
  clicks: number;
}

export interface Stats {
  totalViews: number;
  totalClicks: number;
  totalLinks: number;
  activeLinks: number;
  perLink: LinkStat[];
  daily: DayStat[];
}

/** A marker shown next to the display name. */
export interface Badge {
  id: 'verified' | 'owner' | 'admin';
  label: string;
  /** Tooltip explaining why the badge is there. */
  title: string;
}

export type Role = 'owner' | 'admin' | 'member';

/** Answer from GET /api/session — public, always 200. */
export interface SessionInfo {
  authenticated: boolean;
  isAdmin: boolean;
  username?: string;
  role?: Role;
  expiresIn?: number;
}

/** An account as the admin panel sees it. */
export interface AdminUser {
  id: number;
  username: string;
  role: Role;
  verifiedAt?: string;
  verifiedBy?: string;
  createdAt: string;
  badges: Badge[];
  ownsPage: boolean;
  /** True when the signed-in account may verify, reset or delete this one. */
  canManage: boolean;
  /** True when the signed-in account may promote or demote this one. */
  canChangeRole: boolean;
  /** True when Verified came from the view threshold rather than a grant. */
  autoVerified: boolean;
}

export interface UsersPayload {
  users: AdminUser[];
  threshold: number;
  views: number;
  actor: { username: string; role: Role };
}

/** Everything the owner panel can change. */
export interface SiteSettings {
  headline: string;
  lede: string;
  verifiedThreshold: number;
  maintenance: boolean;
  maintenanceMessage: string;
  indexing: boolean;
}

export interface SettingsPayload {
  settings: SiteSettings;
  limits: {
    headline: number;
    lede: number;
    maintenanceMessage: number;
    minThreshold: number;
    maxThreshold: number;
  };
}

/** Error envelope shared by every endpoint. */
export interface ApiErrorBody {
  error: string;
  message: string;
  fields?: Record<string, string>;
}
