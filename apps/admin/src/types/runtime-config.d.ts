export {};

declare global {
  interface Window {
    __ZOKING_ADMIN_CONFIG__?: {
      apiBaseUrl?: string;
      siteBaseUrl?: string;
    };
  }
}
