import { Navigate, Route, Routes } from 'react-router-dom';
import AppLayout from './layouts/AppLayout';
import Home from './pages/Home';
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Models from './pages/Models';
import Tokens from './pages/Tokens';
import UsageLogs from './pages/UsageLogs';
import Billing from './pages/Billing';
import Playground from './pages/Playground';
import TaskLogs from './pages/TaskLogs';

export default function App() {
  return (
    <Routes>
      <Route path='/' element={<Home />} />
      <Route path='/login' element={<Login />} />
      <Route path='/console' element={<AppLayout />}>
        <Route index element={<Dashboard />} />
        <Route path='models' element={<Models />} />
        <Route path='tokens' element={<Tokens />} />
        <Route path='usage' element={<UsageLogs />} />
        <Route path='tasklog' element={<TaskLogs />} />
        <Route path='billing' element={<Billing />} />
        <Route path='playground' element={<Playground />} />
        <Route path='settings' element={<Navigate to='/console' replace />} />
      </Route>
      <Route path='*' element={<Navigate to='/' replace />} />
    </Routes>
  );
}
