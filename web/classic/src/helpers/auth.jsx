/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { history } from './history';
import { API } from './api';
import {
  getStoredUser,
  hasAnyManagementPermission,
} from './managementPermissions';

export function authHeader() {
  // return authorization header with jwt token
  let user = JSON.parse(localStorage.getItem('user'));

  if (user && user.token) {
    return { Authorization: 'Bearer ' + user.token };
  } else {
    return {};
  }
}

export const AuthRedirect = ({ children }) => {
  const user = localStorage.getItem('user');

  if (user) {
    return <Navigate to='/console' replace />;
  }

  return children;
};

function PrivateRoute({ children }) {
  if (!localStorage.getItem('user')) {
    return <Navigate to='/login' state={{ from: history.location }} />;
  }
  return children;
}

export function AdminRoute({ children }) {
  const raw = localStorage.getItem('user');
  if (!raw) {
    return <Navigate to='/login' state={{ from: history.location }} />;
  }
  try {
    const user = JSON.parse(raw);
    if (user && typeof user.role === 'number' && user.role >= 10) {
      return children;
    }
  } catch (e) {
    // ignore
  }
  return <Navigate to='/forbidden' replace />;
}

export function ManagementPermissionRoute({ children, permissions }) {
  const storedUser = getStoredUser();
  const [state, setState] = useState(() => {
    if (!storedUser) return 'logged-out';
    if (hasAnyManagementPermission(permissions, storedUser)) return 'allowed';
    return Array.isArray(storedUser.management_permissions)
      ? 'forbidden'
      : 'loading';
  });

  useEffect(() => {
    if (state !== 'loading') return;
    let active = true;
    API.get('/api/user/self')
      .then((res) => {
        if (!active) return;
        if (!res.data?.success || !res.data?.data) {
          setState('forbidden');
          return;
        }
        const user = res.data.data;
        localStorage.setItem('user', JSON.stringify(user));
        setState(
          hasAnyManagementPermission(permissions, user)
            ? 'allowed'
            : 'forbidden',
        );
      })
      .catch(() => {
        if (active) setState('forbidden');
      });
    return () => {
      active = false;
    };
  }, [permissions, state]);

  if (state === 'logged-out') {
    return <Navigate to='/login' state={{ from: history.location }} />;
  }
  if (state === 'allowed') return children;
  if (state === 'loading') return null;
  return <Navigate to='/forbidden' replace />;
}

export { PrivateRoute };
