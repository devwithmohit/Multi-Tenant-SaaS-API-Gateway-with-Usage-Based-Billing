import React from 'react';
import { BrowserRouter, Routes, Route, Navigate, Link, useLocation } from 'react-router-dom';
import { LayoutDashboard, Key, FileText, LogOut } from 'lucide-react';
import Login from './pages/Login';
import UsageDashboard from './pages/UsageDashboard';
import APIKeys from './pages/APIKeys';
import Invoices from './pages/Invoices';
import { apiClient } from './api/client';

const App: React.FC = () => {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/*" element={<ProtectedLayout />} />
      </Routes>
    </BrowserRouter>
  );
};

const ProtectedLayout: React.FC = () => {
  const token = localStorage.getItem('auth_token');
  const user = JSON.parse(localStorage.getItem('user') || '{}');

  if (!token) {
    return <Navigate to="/login" replace />;
  }

  const handleLogout = () => {
    apiClient.logout();
    window.location.href = '/login';
  };

  return (
    <div className="flex h-screen bg-gray-100">
      {/* Sidebar */}
      <div className="w-64 bg-white shadow-lg">
        <div className="p-6">
          <h1 className="text-2xl font-bold text-primary-600">Billing Dashboard</h1>
          <p className="text-sm text-gray-500 mt-1">{user.email}</p>
        </div>
        <nav className="mt-6">
          <NavLink to="/dashboard" icon={<LayoutDashboard />} label="Usage Dashboard" />
          <NavLink to="/api-keys" icon={<Key />} label="API Keys" />
          <NavLink to="/invoices" icon={<FileText />} label="Invoices" />
        </nav>
        <div className="absolute bottom-0 w-64 p-4 border-t border-gray-200">
          <button
            onClick={handleLogout}
            className="flex items-center gap-2 w-full px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition"
          >
            <LogOut className="w-5 h-5" />
            Sign Out
          </button>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<UsageDashboard />} />
          <Route path="/api-keys" element={<APIKeys />} />
          <Route path="/invoices" element={<Invoices />} />
        </Routes>
      </div>
    </div>
  );
};

const NavLink: React.FC<{ to: string; icon: React.ReactNode; label: string }> = ({ to, icon, label }) => {
  const location = useLocation();
  const isActive = location.pathname === to;

  return (
    <Link
      to={to}
      className={`flex items-center gap-3 px-6 py-3 transition ${
        isActive
          ? 'bg-primary-50 text-primary-600 border-r-4 border-primary-600'
          : 'text-gray-700 hover:bg-gray-50'
      }`}
    >
      <div className="w-5 h-5">{icon}</div>
      <span className="font-medium">{label}</span>
    </Link>
  );
};

export default App;
