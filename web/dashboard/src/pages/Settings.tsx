import React, { useState, useEffect } from 'react';
import { Settings as SettingsIcon, User, Building, Shield, Key, AlertTriangle, CheckCircle } from 'lucide-react';
import { apiClient } from '../api/client';

// Sprint 8.6: Settings Page
// Covers: profile settings, API key limits, plan upgrade, GDPR data export

interface OrgProfile {
  id: string;
  name: string;
  billing_email: string;
  plan_tier: string;
}

type Tab = 'profile' | 'plan' | 'security' | 'gdpr';

const PLAN_TIERS = ['free', 'starter', 'growth', 'business', 'enterprise'];

const Settings: React.FC = () => {
  const [activeTab, setActiveTab] = useState<Tab>('profile');
  const [org, setOrg] = useState<OrgProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // Profile form
  const [orgName, setOrgName] = useState('');
  const [billingEmail, setBillingEmail] = useState('');

  // Plan form
  const [selectedPlan, setSelectedPlan] = useState('');
  const [isAnnual, setIsAnnual] = useState(false);

  // GDPR
  const [gdprExporting, setGdprExporting] = useState(false);
  const [gdprDeleting, setGdprDeleting] = useState(false);

  useEffect(() => { loadOrg(); }, []);

  const loadOrg = async () => {
    try {
      setLoading(true);
      const res = await (apiClient as any).get('/api/v1/organizations/me');
      const data = res.data;
      setOrg(data);
      setOrgName(data.name || '');
      setBillingEmail(data.billing_email || '');
      setSelectedPlan(data.plan_tier || 'free');
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to load organization settings');
    } finally {
      setLoading(false);
    }
  };

  const showFeedback = (msg: string, isError = false) => {
    if (isError) setError(msg);
    else setSuccess(msg);
    setTimeout(() => { setError(''); setSuccess(''); }, 4000);
  };

  const saveProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await (apiClient as any).put('/api/v1/organizations/me', { name: orgName, billing_email: billingEmail });
      showFeedback('Profile updated successfully');
    } catch (err: any) {
      showFeedback(err.response?.data?.message || 'Failed to save profile', true);
    }
  };

  const updatePlan = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await (apiClient as any).put('/api/v1/organizations/plan', { plan_tier: selectedPlan, is_annual: isAnnual });
      showFeedback(`Plan updated to ${selectedPlan} (${isAnnual ? 'annual' : 'monthly'})`);
      loadOrg();
    } catch (err: any) {
      showFeedback(err.response?.data?.message || 'Failed to update plan', true);
    }
  };

  const exportData = async () => {
    setGdprExporting(true);
    try {
      const res = await (apiClient as any).post('/api/v1/gdpr/export', {});
      const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `data-export-${new Date().toISOString().slice(0,10)}.json`;
      a.click();
      URL.revokeObjectURL(url);
      showFeedback('Data export downloaded');
    } catch (err: any) {
      showFeedback(err.response?.data?.message || 'Failed to export data', true);
    } finally {
      setGdprExporting(false);
    }
  };

  const deleteAccount = async () => {
    if (!window.confirm('Are you sure you want to delete your account? This action is IRREVERSIBLE and will delete all your data.')) return;
    setGdprDeleting(true);
    try {
      await (apiClient as any).delete('/api/v1/gdpr/delete');
      window.location.href = '/login';
    } catch (err: any) {
      showFeedback(err.response?.data?.message || 'Failed to delete account', true);
      setGdprDeleting(false);
    }
  };

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: 'profile', label: 'Profile', icon: <Building className="w-4 h-4" /> },
    { key: 'plan', label: 'Plan & Billing', icon: <Key className="w-4 h-4" /> },
    { key: 'security', label: 'Security', icon: <Shield className="w-4 h-4" /> },
    { key: 'gdpr', label: 'Data & Privacy', icon: <User className="w-4 h-4" /> },
  ];

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-600" />
      </div>
    );
  }

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="flex items-center gap-3 mb-6">
        <SettingsIcon className="w-7 h-7 text-primary-600" />
        <h1 className="text-2xl font-bold text-gray-900">Settings</h1>
      </div>

      {/* Feedback banners */}
      {success && (
        <div className="mb-4 p-3 bg-green-50 border border-green-200 rounded-lg flex items-center gap-2">
          <CheckCircle className="w-4 h-4 text-green-600 flex-shrink-0" />
          <p className="text-sm text-green-800">{success}</p>
        </div>
      )}
      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-red-600 flex-shrink-0" />
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      <div className="flex gap-6">
        {/* Tab sidebar */}
        <nav className="w-48 flex-shrink-0">
          <ul className="space-y-1">
            {tabs.map(tab => (
              <li key={tab.key}>
                <button
                  onClick={() => setActiveTab(tab.key)}
                  className={`w-full flex items-center gap-2 px-3 py-2 rounded-lg text-sm transition ${
                    activeTab === tab.key
                      ? 'bg-primary-50 text-primary-700 font-medium'
                      : 'text-gray-600 hover:bg-gray-50'
                  }`}
                >
                  {tab.icon} {tab.label}
                </button>
              </li>
            ))}
          </ul>
        </nav>

        {/* Tab content */}
        <div className="flex-1 bg-white border border-gray-200 rounded-xl shadow-sm p-6">
          {activeTab === 'profile' && (
            <form onSubmit={saveProfile} className="space-y-4">
              <h2 className="text-lg font-semibold text-gray-900 mb-4">Organization Profile</h2>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Organization Name</label>
                <input type="text" value={orgName} onChange={e => setOrgName(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Billing Email</label>
                <input type="email" value={billingEmail} onChange={e => setBillingEmail(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500" />
              </div>
              <button type="submit"
                className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition">
                Save Changes
              </button>
            </form>
          )}

          {activeTab === 'plan' && (
            <form onSubmit={updatePlan} className="space-y-4">
              <h2 className="text-lg font-semibold text-gray-900 mb-4">Plan & Billing</h2>
              <p className="text-sm text-gray-600">Current plan: <span className="font-medium capitalize">{org?.plan_tier}</span></p>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Select Plan</label>
                <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
                  {PLAN_TIERS.map(tier => (
                    <button key={tier} type="button" onClick={() => setSelectedPlan(tier)}
                      className={`border rounded-lg p-3 text-sm capitalize transition ${
                        selectedPlan === tier
                          ? 'border-primary-500 bg-primary-50 text-primary-700 font-medium'
                          : 'border-gray-200 text-gray-700 hover:border-gray-300'
                      }`}>
                      {tier}
                    </button>
                  ))}
                </div>
              </div>
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={isAnnual} onChange={e => setIsAnnual(e.target.checked)}
                  className="rounded text-primary-600" />
                <span className="text-sm text-gray-700">Annual billing (save ~17%)</span>
              </label>
              <button type="submit"
                className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition">
                Update Plan
              </button>
            </form>
          )}

          {activeTab === 'security' && (
            <div className="space-y-4">
              <h2 className="text-lg font-semibold text-gray-900 mb-4">Security</h2>
              <div className="rounded-lg bg-blue-50 border border-blue-200 p-4">
                <p className="text-sm text-blue-800">
                  <strong>API Key Security:</strong> All API keys are stored as SHA-256 hashes.
                  The raw key is only shown once at creation time.
                </p>
              </div>
              <div className="rounded-lg bg-blue-50 border border-blue-200 p-4">
                <p className="text-sm text-blue-800">
                  <strong>Session Security:</strong> JWT tokens expire after 24 hours.
                  Your session is automatically invalidated when you log out.
                </p>
              </div>
              <div className="rounded-lg bg-yellow-50 border border-yellow-200 p-4">
                <p className="text-sm text-yellow-800">
                  <strong>Tenant Isolation:</strong> All data is isolated at the database level using Row-Level Security (RLS).
                  Your data is never accessible to other organizations.
                </p>
              </div>
            </div>
          )}

          {activeTab === 'gdpr' && (
            <div className="space-y-6">
              <h2 className="text-lg font-semibold text-gray-900 mb-4">Data & Privacy (GDPR)</h2>

              <div className="border border-gray-200 rounded-lg p-4">
                <h3 className="font-medium text-gray-900 mb-2">Export Your Data</h3>
                <p className="text-sm text-gray-600 mb-3">
                  Download all data associated with your organization in JSON format.
                </p>
                <button onClick={exportData} disabled={gdprExporting}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 transition">
                  {gdprExporting ? 'Exporting...' : 'Export Data'}
                </button>
              </div>

              <div className="border border-red-200 rounded-lg p-4 bg-red-50">
                <h3 className="font-medium text-red-900 mb-2">Delete Account</h3>
                <p className="text-sm text-red-700 mb-3">
                  Permanently delete your organization and all associated data.
                  This action is <strong>irreversible</strong>.
                </p>
                <button onClick={deleteAccount} disabled={gdprDeleting}
                  className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 transition">
                  {gdprDeleting ? 'Deleting...' : 'Delete My Account'}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default Settings;
