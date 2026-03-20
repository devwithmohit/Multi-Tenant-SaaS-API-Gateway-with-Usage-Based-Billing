import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Login from './pages/Login';
import Register from './pages/Register';
import UsageDashboard from './pages/UsageDashboard';
import APIKeys from './pages/APIKeys';
import Invoices from './pages/Invoices';
import Alerts from './pages/Alerts';
import Webhooks from './pages/Webhooks';
import Settings from './pages/Settings';
import Layout from './components/Layout';
import { ToastProvider } from './components/Toast';

const App: React.FC = () => {
  return (
    <ToastProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/*" element={<ProtectedLayout />} />
        </Routes>
      </BrowserRouter>
    </ToastProvider>
  );
};

const ProtectedLayout: React.FC = () => {
  const token = localStorage.getItem('auth_token');

  if (!token) {
    return <Navigate to="/login" replace />;
  }

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<UsageDashboard />} />
        <Route path="/api-keys" element={<APIKeys />} />
        <Route path="/invoices" element={<Invoices />} />
        <Route path="/alerts" element={<Alerts />} />
        <Route path="/webhooks" element={<Webhooks />} />
        <Route path="/settings" element={<Settings />} />
      </Routes>
    </Layout>
  );
};

export default App;
