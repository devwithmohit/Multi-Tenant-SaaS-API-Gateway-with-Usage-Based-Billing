import React, { useEffect, useState } from 'react';
import { TrendingUp, DollarSign, Activity, Clock } from 'lucide-react';
import { apiClient } from '../api/client';
import UsageChart from '../components/UsageChart';
import RateLimitGauge from '../components/RateLimitGauge';
import type { CurrentUsageResponse, UsageHistoryResponse } from '../types';

const UsageDashboard: React.FC = () => {
  const [currentUsage, setCurrentUsage] = useState<CurrentUsageResponse | null>(null);
  const [usageHistory, setUsageHistory] = useState<UsageHistoryResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedDays, setSelectedDays] = useState(30);

  useEffect(() => {
    loadData();
  }, [selectedDays]);

  const loadData = async () => {
    try {
      setLoading(true);
      const [current, history] = await Promise.all([
        apiClient.getCurrentUsage(),
        apiClient.getUsageHistory(selectedDays),
      ]);
      setCurrentUsage(current);
      setUsageHistory(history);
      setError(null);
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to load usage data');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-6">
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <p className="text-red-800">{error}</p>
          <button
            onClick={loadData}
            className="mt-2 text-sm text-red-600 hover:text-red-800 underline"
          >
            Try again
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Usage Dashboard</h1>
          <p className="text-gray-500 mt-1">Monitor your API usage and costs</p>
        </div>
        <select
          value={selectedDays}
          onChange={(e) => setSelectedDays(Number(e.target.value))}
          className="px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
        >
          <option value={7}>Last 7 days</option>
          <option value={30}>Last 30 days</option>
          <option value={90}>Last 90 days</option>
        </select>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <StatCard
          icon={<DollarSign className="w-6 h-6" />}
          title="Today's Cost"
          value={`$${currentUsage?.total_cost.toFixed(2) || '0.00'}`}
          color="text-green-600"
          bgColor="bg-green-50"
        />
        <StatCard
          icon={<TrendingUp className="w-6 h-6" />}
          title="Total Cost (Period)"
          value={`$${usageHistory?.total_cost.toFixed(2) || '0.00'}`}
          color="text-blue-600"
          bgColor="bg-blue-50"
        />
        <StatCard
          icon={<Activity className="w-6 h-6" />}
          title="Active Metrics"
          value={currentUsage?.metrics.length || 0}
          color="text-purple-600"
          bgColor="bg-purple-50"
        />
        <StatCard
          icon={<Clock className="w-6 h-6" />}
          title="Last Updated"
          value={currentUsage ? new Date(currentUsage.updated_at).toLocaleTimeString() : 'N/A'}
          color="text-gray-600"
          bgColor="bg-gray-50"
        />
      </div>

      {/* Usage Chart */}
      {usageHistory && usageHistory.daily_usage.length > 0 && (
        <UsageChart
          data={usageHistory.daily_usage}
          title={`Usage Trends - Last ${selectedDays} Days`}
        />
      )}

      {/* Current Metrics */}
      {currentUsage && (
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">Today's Usage Metrics</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {currentUsage.metrics.map((metric) => (
              <div key={metric.metric_name} className="border border-gray-200 rounded-lg p-4">
                <div className="flex items-center justify-between mb-2">
                  <h3 className="font-medium text-gray-900">
                    {metric.metric_name.replace(/_/g, ' ').toUpperCase()}
                  </h3>
                  <span className="text-sm text-gray-500">${metric.cost.toFixed(4)}</span>
                </div>
                <p className="text-2xl font-bold text-gray-900">
                  {metric.total_value.toLocaleString()} <span className="text-sm font-normal text-gray-500">{metric.unit}</span>
                </p>
                <p className="text-sm text-gray-500 mt-1">{metric.count} events</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Rate Limit Gauges */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {currentUsage?.metrics.map((metric) => (
          <RateLimitGauge
            key={metric.metric_name}
            current={metric.total_value}
            limit={getMetricLimit(metric.metric_name)}
            label={metric.metric_name.replace(/_/g, ' ').toUpperCase()}
            unit={metric.unit}
          />
        ))}
      </div>
    </div>
  );
};

// Helper component for stat cards
const StatCard: React.FC<{
  icon: React.ReactNode;
  title: string;
  value: string | number;
  color: string;
  bgColor: string;
}> = ({ icon, title, value, color, bgColor }) => (
  <div className="bg-white rounded-lg shadow p-6">
    <div className="flex items-center gap-3">
      <div className={`p-3 rounded-lg ${bgColor}`}>
        <div className={color}>{icon}</div>
      </div>
      <div>
        <p className="text-sm text-gray-500">{title}</p>
        <p className="text-2xl font-bold text-gray-900">{value}</p>
      </div>
    </div>
  </div>
);

// Helper function to get metric limits (would come from config/subscription in production)
const getMetricLimit = (metricName: string): number => {
  const limits: Record<string, number> = {
    api_requests: 1000000,
    data_transfer_gb: 1000,
    storage_gb: 500,
    compute_hours: 1000,
  };
  return limits[metricName] || 10000;
};

export default UsageDashboard;
