import React, { useState, useEffect } from 'react';
import { Webhook, Plus, Trash2, AlertTriangle, Edit2 } from 'lucide-react';
import { apiClient } from '../api/client';

// Sprint 8.7: Webhook Configuration Page
// Connects to GET/POST/PUT/DELETE /api/v1/webhooks

interface WebhookEndpoint {
  id: string;
  url: string;
  events: string[];
  is_active: boolean;
  created_at: string;
}

const EVENT_TYPES = [
  'invoice.created', 'invoice.paid', 'payment.failed',
  'usage.threshold.80', 'usage.threshold.100', 'key.revoked',
];

const Webhooks: React.FC = () => {
  const [webhooks, setWebhooks] = useState<WebhookEndpoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [formUrl, setFormUrl] = useState('');
  const [selectedEvents, setSelectedEvents] = useState<string[]>([]);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => { loadWebhooks(); }, []);

  const loadWebhooks = async () => {
    try {
      setLoading(true);
      const res = await (apiClient as any).get('/api/v1/webhooks');
      setWebhooks(res.data.webhooks || []);
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to load webhooks');
    } finally {
      setLoading(false);
    }
  };

  const toggleEvent = (event: string) => {
    setSelectedEvents(prev =>
      prev.includes(event) ? prev.filter(e => e !== event) : [...prev, event]
    );
  };

  const createWebhook = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formUrl || selectedEvents.length === 0) {
      setError('URL and at least one event type are required');
      return;
    }
    setSubmitting(true);
    try {
      await (apiClient as any).post('/api/v1/webhooks', { url: formUrl, events: selectedEvents });
      setShowForm(false);
      setFormUrl('');
      setSelectedEvents([]);
      setError('');
      await loadWebhooks();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to create webhook');
    } finally {
      setSubmitting(false);
    }
  };

  const deleteWebhook = async (id: string) => {
    if (!window.confirm('Delete this webhook endpoint?')) return;
    try {
      await (apiClient as any).delete(`/api/v1/webhooks/${id}`);
      await loadWebhooks();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to delete webhook');
    }
  };

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Webhook className="w-7 h-7 text-primary-600" />
          <h1 className="text-2xl font-bold text-gray-900">Webhook Endpoints</h1>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition"
        >
          <Plus className="w-4 h-4" /> New Endpoint
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 border border-red-200 rounded-lg flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-red-600 flex-shrink-0" />
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {showForm && (
        <form onSubmit={createWebhook} className="mb-6 p-5 bg-white border border-gray-200 rounded-xl shadow-sm space-y-4">
          <h2 className="font-semibold text-gray-900">Add Webhook Endpoint</h2>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Endpoint URL</label>
            <input
              type="url" required
              value={formUrl}
              onChange={e => setFormUrl(e.target.value)}
              placeholder="https://your-server.com/webhook"
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Events to receive</label>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-2">
              {EVENT_TYPES.map(evt => (
                <label key={evt} className="flex items-center gap-2 text-sm cursor-pointer">
                  <input
                    type="checkbox"
                    checked={selectedEvents.includes(evt)}
                    onChange={() => toggleEvent(evt)}
                    className="rounded text-primary-600"
                  />
                  <span className="text-gray-700">{evt}</span>
                </label>
              ))}
            </div>
          </div>
          <div className="flex gap-3">
            <button type="submit" disabled={submitting}
              className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 disabled:opacity-50 transition">
              {submitting ? 'Creating...' : 'Create Endpoint'}
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
      ) : webhooks.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          <Webhook className="w-12 h-12 mx-auto mb-3 text-gray-300" />
          <p>No webhook endpoints configured. Add one to receive event notifications.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {webhooks.map(wh => (
            <div key={wh.id} className="p-4 bg-white border border-gray-200 rounded-xl shadow-sm">
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3 min-w-0">
                  <div className={`w-2 h-2 rounded-full flex-shrink-0 mt-1 ${wh.is_active ? 'bg-green-500' : 'bg-gray-400'}`} />
                  <div className="min-w-0">
                    <p className="font-mono text-sm font-medium text-gray-900 truncate">{wh.url}</p>
                    <div className="flex flex-wrap gap-1 mt-1">
                      {wh.events.map(evt => (
                        <span key={evt} className="px-2 py-0.5 bg-primary-50 text-primary-700 text-xs rounded-full">{evt}</span>
                      ))}
                    </div>
                  </div>
                </div>
                <button onClick={() => deleteWebhook(wh.id)}
                  className="ml-2 p-2 text-red-500 hover:bg-red-50 rounded-lg transition flex-shrink-0">
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default Webhooks;
