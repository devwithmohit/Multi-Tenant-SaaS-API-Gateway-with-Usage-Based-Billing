export interface User {
  id: string;
  email: string;
  organization_id: string;
  role: string;
  first_name: string;
  last_name: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  token_type: string;
  expires_in: number;
  user: User;
}

export interface UsageMetricSummary {
  metric_name: string;
  total_value: number;
  unit: string;
  count: number;
  cost: number;
  description?: string;
}

export interface CurrentUsageResponse {
  organization_id: string;
  date: string;
  metrics: UsageMetricSummary[];
  total_cost: number;
  updated_at: string;
}

export interface DailyUsageSummary {
  date: string;
  metrics: UsageMetricSummary[];
  cost: number;
}

export interface UsageHistoryResponse {
  organization_id: string;
  start_date: string;
  end_date: string;
  daily_usage: DailyUsageSummary[];
  total_cost: number;
}

export interface APIKey {
  id: string;
  organization_id: string;
  name: string;
  key_prefix: string;
  last_used_at?: string;
  created_at: string;
  expires_at?: string;
  revoked_at?: string;
  status: 'active' | 'revoked' | 'expired';
  created_by: string;
}

export interface CreateAPIKeyRequest {
  name: string;
  expires_at?: string;
}

export interface CreateAPIKeyResponse {
  api_key: APIKey;
  full_key: string;
  message: string;
}

export interface Invoice {
  id: string;
  invoice_number: string;
  organization_id: string;
  customer_name: string;
  customer_email: string;
  billing_period_start: string;
  billing_period_end: string;
  status: 'draft' | 'pending' | 'paid' | 'failed' | 'refunded' | 'voided';
  subtotal: number;
  tax: number;
  total: number;
  currency: string;
  due_date: string;
  paid_at?: string;
  pdf_url?: string;
  stripe_invoice_id?: string;
  created_at: string;
  updated_at: string;
}

export interface InvoiceLineItem {
  id: string;
  invoice_id: string;
  description: string;
  quantity: number;
  unit_price: number;
  amount: number;
  metric_name?: string;
}

export interface InvoiceListResponse {
  invoices: Invoice[];
  total_count: number;
  page: number;
  page_size: number;
}
