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
import React from 'react';
import { Tabs } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import ModelsTable from '../../components/table/models';
import ChatModelsTable from '../../components/table/chat-models';
import PlaygroundModelRulesTable from '../../components/table/playground-model-rules';
import {
  isAdmin,
  MANAGEMENT_PERMISSION,
  hasManagementPermission,
} from '../../helpers';

const ModelPage = () => {
  const { t } = useTranslation();
  const adminAccess = isAdmin();
  const canManageChatModels = hasManagementPermission(
    MANAGEMENT_PERMISSION.CHAT_MODELS,
  );

  return (
    <div className='mt-[60px] px-2'>
      <Tabs type='card' defaultActiveKey={adminAccess ? 'marketplace' : 'chat'}>
        {adminAccess ? (
          <Tabs.TabPane tab={t('模型广场管理')} itemKey='marketplace'>
            <ModelsTable />
          </Tabs.TabPane>
        ) : null}
        {canManageChatModels ? (
          <Tabs.TabPane tab={t('对话模型管理')} itemKey='chat'>
            <ChatModelsTable />
          </Tabs.TabPane>
        ) : null}
        {adminAccess ? (
          <Tabs.TabPane tab={t('操练场模型规则')} itemKey='playground-rules'>
            <PlaygroundModelRulesTable />
          </Tabs.TabPane>
        ) : null}
      </Tabs>
    </div>
  );
};

export default ModelPage;
