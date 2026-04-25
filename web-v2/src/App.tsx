import { Navigate, Route, Routes } from 'react-router-dom';
import AppLayout from './layouts/AppLayout';
import Home from './pages/Home';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Channels from './pages/Channels';
import Tokens from './pages/Tokens';
import UsageLogs from './pages/UsageLogs';
import Billing from './pages/Billing';

export default function App() {
  return (
    <Routes>
      <Route path='/' element={<Home />} />
      <Route path='/login' element={<Login />} />
      <Route path='/console' element={<AppLayout />}>
        <Route index element={<Dashboard />} />
        <Route path='channels' element={<Channels />} />
        <Route path='tokens' element={<Tokens />} />
        <Route path='usage' element={<UsageLogs />} />
        <Route path='billing' element={<Billing />} />
        <Route path='settings' element={<Navigate to='/console' replace />} />
      </Route>
      <Route path='*' element={<Navigate to='/' replace />} />
    </Routes>
  );
}
