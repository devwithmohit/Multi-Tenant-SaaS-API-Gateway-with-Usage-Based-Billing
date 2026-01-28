import axios, { AxiosInstance, AxiosError } from 'axios';
import type {
  LoginRequest,
  LoginResponse,
  CurrentUsageResponse,
  UsageHistoryResponse,
  APIKey,
  CreateAPIKeyRequest,
  CreateAPIKeyResponse,
  InvoiceListResponse,
  Invoice,
  InvoiceLineItem,
} from '../types';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

class APIClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add request interceptor to attach JWT token
    this.client.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem('auth_token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    // Add response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => response,
      (error: AxiosError) => {
        if (error.response?.status === 401) {
          // Token expired or invalid - redirect to login
          localStorage.removeItem('auth_token');
          localStorage.removeItem('user');
          window.location.href = '/login';
        }
        return Promise.reject(error);
      }
    );
  }

  // Authentication
  async login(credentials: LoginRequest): Promise<LoginResponse> {
    const { data } = await this.client.post<LoginResponse>('/api/v1/auth/login', credentials);
    // Store token in localStorage
    localStorage.setItem('auth_token', data.token);
    localStorage.setItem('user', JSON.stringify(data.user));
    return data;
  }

  logout() {
    localStorage.removeItem('auth_token');
    localStorage.removeItem('user');
  }

  async validateToken(): Promise<{ valid: boolean; user: any }> {
    const { data } = await this.client.get('/api/v1/auth/validate');
    return data;
  }

  // Usage endpoints
  async getCurrentUsage(): Promise<CurrentUsageResponse> {
    const { data } = await this.client.get<CurrentUsageResponse>('/api/v1/usage/current');
    return data;
  }

  async getUsageHistory(days: number = 90): Promise<UsageHistoryResponse> {
    const { data } = await this.client.get<UsageHistoryResponse>('/api/v1/usage/history', {
      params: { days },
    });
    return data;
  }

  async getUsageByMetric(metric: string, days: number = 30): Promise<any> {
    const { data } = await this.client.get('/api/v1/usage/metrics', {
      params: { metric, days },
    });
    return data;
  }

  // API Key endpoints
  async listAPIKeys(): Promise<{ api_keys: APIKey[]; count: number }> {
    const { data } = await this.client.get('/api/v1/apikeys');
    return data;
  }

  async createAPIKey(request: CreateAPIKeyRequest): Promise<CreateAPIKeyResponse> {
    const { data } = await this.client.post<CreateAPIKeyResponse>('/api/v1/apikeys', request);
    return data;
  }

  async getAPIKey(id: string): Promise<APIKey> {
    const { data } = await this.client.get<APIKey>(`/api/v1/apikeys/${id}`);
    return data;
  }

  async revokeAPIKey(id: string): Promise<{ success: boolean; message: string }> {
    const { data } = await this.client.delete(`/api/v1/apikeys/${id}`);
    return data;
  }

  // Invoice endpoints
  async listInvoices(page: number = 1, pageSize: number = 20): Promise<InvoiceListResponse> {
    const { data } = await this.client.get<InvoiceListResponse>('/api/v1/invoices', {
      params: { page, page_size: pageSize },
    });
    return data;
  }

  async getInvoice(id: string): Promise<{ invoice: Invoice; line_items: InvoiceLineItem[] }> {
    const { data } = await this.client.get(`/api/v1/invoices/${id}`);
    return data;
  }

  getInvoicePDFUrl(id: string): string {
    return `${API_BASE_URL}/api/v1/invoices/${id}/pdf`;
  }
}

export const apiClient = new APIClient();
