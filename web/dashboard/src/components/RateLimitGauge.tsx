import React from 'react';
import { Activity } from 'lucide-react';

interface RateLimitGaugeProps {
  current: number;
  limit: number;
  label: string;
  unit: string;
}

const RateLimitGauge: React.FC<RateLimitGaugeProps> = ({ current, limit, label, unit }) => {
  const percentage = Math.min((current / limit) * 100, 100);
  const isNearLimit = percentage >= 80;
  const isAtLimit = percentage >= 100;

  const getColor = () => {
    if (isAtLimit) return 'bg-red-500';
    if (isNearLimit) return 'bg-yellow-500';
    return 'bg-primary-500';
  };

  const getTextColor = () => {
    if (isAtLimit) return 'text-red-600';
    if (isNearLimit) return 'text-yellow-600';
    return 'text-primary-600';
  };

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Activity className="w-5 h-5 text-gray-500" />
          <h3 className="text-lg font-semibold">{label}</h3>
        </div>
        <span className={`text-sm font-medium ${getTextColor()}`}>
          {percentage.toFixed(1)}%
        </span>
      </div>

      {/* Progress bar */}
      <div className="mb-4">
        <div className="w-full bg-gray-200 rounded-full h-3">
          <div
            className={`h-3 rounded-full transition-all duration-300 ${getColor()}`}
            style={{ width: `${percentage}%` }}
          />
        </div>
      </div>

      {/* Values */}
      <div className="flex items-center justify-between text-sm">
        <span className="text-gray-600">
          Current: <span className="font-semibold text-gray-900">{current.toLocaleString()} {unit}</span>
        </span>
        <span className="text-gray-600">
          Limit: <span className="font-semibold text-gray-900">{limit.toLocaleString()} {unit}</span>
        </span>
      </div>

      {/* Warning message */}
      {isNearLimit && (
        <div className={`mt-4 p-3 rounded-md ${isAtLimit ? 'bg-red-50 text-red-800' : 'bg-yellow-50 text-yellow-800'}`}>
          <p className="text-sm font-medium">
            {isAtLimit
              ? '⚠️ Limit reached! Additional usage may be throttled.'
              : '⚠️ Approaching limit. Consider upgrading your plan.'}
          </p>
        </div>
      )}
    </div>
  );
};

export default RateLimitGauge;
