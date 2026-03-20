import React, { useState, useEffect } from 'react';
import { Bell, Plus, Trash2, AlertTriangle } from 'lucide-react';
import { apiClient } from '../api/client';

// Sprint 8.8: Alert Configuration Page
// Connects to GET/POST/PUT/DELETE /api/v1/alerts

interface AlertConfig {
  id: string;
  alert_type: string;
  threshold: number;
  channel: string;
  is_active: boolean;
  created_at: string;
}

const ALERT_TYPES = [
  { value: 'usage_threshold', label: 'Usage Threshold (%)' },
  { value: 'cost_threshold', label: 'Cost Threshold ($)' },
  { value: 'error_rate', label: 'Error Rate (%)' },
];

const CHANNELS = ['email', 'webhook', 'in_app'];

const Alerts: React.FC = () => {
  const [alerts, setAlerts] = useState<AlertConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState({ alert_type: 'usage_threshold', threshold: 80, channel: 'email' });
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => { loadAlerts(); }, []);

  const loadAlerts = async () => {
    try {
      setLoading(true);
      const res = await (apiClient as any).get('/api/v1/alerts');
      setAlerts(res.data.alerts || []);
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to load alerts');
    } finally {
      setLoading(false);
    }
  };

  const createAlert = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      await (apiClient as any).post('/api/v1/alerts', formData);
      setShowForm(false);
      setFormData({ alert_type: 'usage_threshold', threshold: 80, channel: 'email' });
      await loadAlerts();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to create alert');
    } finally {
      setSubmitting(false);
    }
  };

  const deleteAlert = async (id: string) => {
    if (!window.confirm('Delete this alert?')) return;
    try {
      await (apiClient as any).delete(`/api/v1/alerts/${id}`);
      await loadAlerts();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to delete alert');
    }
  };

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Bell className="w-7 h-7 text-primary-600" />
          <h1 className="text-2xl font-bold text-gray-900">Alert Configuration</h1>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition"
        >
          <Plus className="w-4 h-4" /> New Alert
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-red-600 flex-shrink-0" />
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {showForm && (
        <form onSubmit={createAlert} className="mb-6 p-5 bg-white border border-gray-200 rounded-xl shadow-sm space-y-4">
          <h2 className="font-semibold text-gray-900">Create Alert</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Alert Type</label>
              <select
                value={formData.alert_type}
                onChange={e => setFormData({ ...formData, alert_type: e.target.value })}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
              >
                {ALERT_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Threshold</label>
              <input
                type="number" min={1} max={10000}
                value={formData.threshold}
                onChange={e => setFormData({ ...formData, threshold: Number(e.target.value) })}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Channel</label>
              <select
                value={formData.channel}
                onChange={e => setFormData({ ...formData, channel: e.target.value })}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
              >
                {CHANNELS.map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
          </div>
          <div className="flex gap-3">
            <button type="submit" disabled={submitting}
              className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition">
              {submitting ? 'Creating...' : 'Create Alert'}
            </button>
            <button type="button" onClick={() => setShowForm(false)}
              className="px-4 py-2 bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 transition">
              Cancel
            </button>
          </div>
        </form>
      )}

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-600" />
        </div>
      ) : alerts.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          <Bell className="w-12 h-12 mx-auto mb-3 text-gray-300" />
          <p>No alerts configured. Create one to get notified about usage thresholds.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {alerts.map(alert => (
            <div key={alert.id} className="flex items-center justify-between p-4 bg-white border border-gray-200 rounded-xl shadow-sm">
              <div className="flex items-center gap-4">
                <div className={`w-2 h-2 rounded-full ${alert.is_active ? 'bg-green-500' : 'bg-gray-400'}`} />
                <div>
                  <p className="font-medium text-gray-900 capitalize">{alert.alert_type.replace(/_/g, ' ')}</p>
                  <p className="text-sm text-gray-500">Threshold: {alert.threshold} · Channel: {alert.channel}</p>
                </div>
              </div>
              <button onClick={() => deleteAlert(alert.id)}
                className="p-2 text-red-500 hover:bg-red-50 rounded-lg transition">
                <Trash2 className="w-4 h-4" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default Alerts;
