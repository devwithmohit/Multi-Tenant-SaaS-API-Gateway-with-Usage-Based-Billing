import React from 'react';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { format } from 'date-fns';

interface UsageChartProps {
  data: Array<{
    date: string;
    cost: number;
    metrics?: Array<{
      metric_name: string;
      total_value: number;
      cost: number;
    }>;
  }>;
  title?: string;
}

const UsageChart: React.FC<UsageChartProps> = ({ data, title = 'Usage Over Time' }) => {
  // Transform data for chart
  const chartData = data.map((item) => ({
    date: format(new Date(item.date), 'MMM dd'),
    cost: item.cost,
    ...item.metrics?.reduce((acc, metric) => ({
      ...acc,
      [metric.metric_name]: metric.total_value,
    }), {}),
  }));

  // Get unique metric names for lines
  const metricNames = Array.from(
    new Set(data.flatMap((item) => item.metrics?.map((m) => m.metric_name) || []))
  );

  const colors = ['#0ea5e9', '#8b5cf6', '#ec4899', '#f59e0b', '#10b981'];

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <h3 className="text-lg font-semibold mb-4">{title}</h3>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="date" />
          <YAxis />
          <Tooltip />
          <Legend />
          <Line
            type="monotone"
            dataKey="cost"
            stroke="#0ea5e9"
            strokeWidth={2}
            name="Cost ($)"
          />
          {metricNames.map((name, index) => (
            <Line
              key={name}
              type="monotone"
              dataKey={name}
              stroke={colors[index % colors.length]}
              strokeWidth={2}
              name={name.replace(/_/g, ' ')}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
};

export default UsageChart;
