export interface TranslationKeys {
  // ====== 通用 ======
  "common.save": string;
  "common.cancel": string;
  "common.close": string;
  "common.delete": string;
  "common.edit": string;
  "common.copy": string;
  "common.refresh": string;
  "common.search": string;
  "common.loading": string;
  "common.noData": string;
  "common.confirm": string;
  "common.apply": string;
  "common.clear": string;
  "common.execute": string;
  "common.test": string;
  "common.back": string;
  "common.create": string;
  "common.noResults": string;
  "common.success": string;
  "common.error": string;
  "common.actions": string;
  "common.commit": string;

  // ====== 菜单/工具栏 ======
  "toolbar.toggleSidebar": string;
  "toolbar.collapseSidebar": string;
  "toolbar.expandSidebar": string;
  "toolbar.newConnection": string;
  "toolbar.sqlQuery": string;
  "toolbar.quickSearch": string;
  "toolbar.aiAssistant": string;
  "toolbar.viewLogs": string;
  "toolbar.switchToLight": string;
  "toolbar.switchToDark": string;
  "toolbar.settings": string;
  "toolbar.switchDatabase": string;

  // ====== 侧边栏 ======
  "sidebar.noConnections": string;
  "sidebar.addConnection": string;
  "sidebar.notConnected": string;
  "sidebar.connect": string;
  "sidebar.disconnect": string;
  "sidebar.editConnection": string;
  "sidebar.tables": string;
  "sidebar.searchPlaceholder": string;

  // ====== 连接对话框 ======
  "connection.newConnection": string;
  "connection.editConnection": string;
  "connection.searchConnections": string;
  "connection.createNew": string;
  "connection.noSaved": string;
  "connection.noMatch": string;
  "connection.name": string;
  "connection.host": string;
  "connection.port": string;
  "connection.user": string;
  "connection.password": string;
  "connection.database": string;
  "connection.databaseFile": string;
  "connection.testConnection": string;
  "connection.testing": string;
  "connection.testSuccess": string;
  "connection.testFailed": string;
  "connection.saveAndConnect": string;

  // ====== 标签页 ======
  "tabs.close": string;
  "tabs.closeOthers": string;
  "tabs.closeRight": string;
  "tabs.closeAll": string;
  "tabs.newQuery": string;

  // ====== 数据表格 ======
  "datagrid.columnFilter": string;
  "datagrid.rawSQL": string;
  "datagrid.addCondition": string;
  "datagrid.value": string;

  // ====== 右键菜单（行） ======
  "contextMenu.previewRow": string;
  "contextMenu.copyCell": string;
  "contextMenu.copyRow": string;
  "contextMenu.copyAsInsert": string;
  "contextMenu.downloadCSV": string;
  "contextMenu.deleteRow": string;

  // ====== 右键菜单（表） ======
  "contextMenu.openInNewTab": string;
  "contextMenu.copyTableName": string;
  "contextMenu.viewData": string;
  "contextMenu.viewStructure": string;
  "contextMenu.viewDDL": string;
  "contextMenu.tableDoc": string;
  "contextMenu.exportData": string;
  "contextMenu.truncateTable": string;
  "contextMenu.dropTable": string;
  "contextMenu.truncateConfirm": string;
  "contextMenu.dropConfirm": string;
  "contextMenu.truncateFailed": string;
  "contextMenu.dropFailed": string;

  // ====== SQL 编辑器 ======
  "editor.execute": string;
  "editor.executeAll": string;
  "editor.format": string;
  "editor.compress": string;
  "editor.unescape": string;
  "editor.aiAssist": string;
  "editor.save": string;
  "editor.copySQL": string;

  // ====== 设置 ======
  "settings.title": string;
  "settings.general": string;
  "settings.aiConfig": string;
  "settings.appearance": string;
  "settings.language": string;

  // ====== 通用设置 ======
  "generalSettings.title": string;
  "generalSettings.description": string;
  "generalSettings.theme": string;
  "generalSettings.themeLight": string;
  "generalSettings.themeDark": string;
  "generalSettings.themeSystem": string;
  "generalSettings.pageSize": string;
  "generalSettings.languageLabel": string;
  "generalSettings.languageDescription": string;
  "generalSettings.layoutMode": string;
  "generalSettings.layoutCompact": string;
  "generalSettings.layoutCompactDesc": string;
  "generalSettings.layoutDefault": string;
  "generalSettings.layoutDefaultDesc": string;

  // ====== AI 设置 ======
  "aiSettings.title": string;
  "aiSettings.description": string;
  "aiSettings.baseURL": string;
  "aiSettings.apiKey": string;
  "aiSettings.model": string;
  "aiSettings.maxTokens": string;
  "aiSettings.temperature": string;
  "aiSettings.customHeaders": string;
  "aiSettings.customHeadersHint": string;
  "aiSettings.headerName": string;
  "aiSettings.headerValue": string;
  "aiSettings.testConnection": string;
  "aiSettings.testingConnection": string;
  "aiSettings.testSuccess": string;
  "aiSettings.testFailed": string;
  "aiSettings.saving": string;
  "aiSettings.saved": string;
  "aiSettings.saveFailed": string;

  // ====== AI 面板 ======
  "ai.title": string;
  "ai.newChat": string;
  "ai.chatHistory": string;
  "ai.clearChat": string;
  "ai.noHistory": string;
  "ai.messages": string;
  "ai.placeholder": string;
  "ai.thinking": string;
  "ai.dbAssistant": string;
  "ai.dbAssistantDesc": string;
  "ai.tryExample1": string;
  "ai.tryExample2": string;
  "ai.tryExample3": string;
  "ai.tryLabel": string;
  "ai.database": string;
  "ai.deleteSession": string;
  "ai.executeSQL": string;
  "ai.queryResult": string;
  "ai.executeSuccess": string;
  "ai.executeError": string;
  "ai.executeFailed": string;
  "ai.noContent": string;
  "ai.requestFailed": string;

  // ====== 命令面板 ======
  "command.searchPlaceholder": string;
  "command.newConnection": string;
  "command.openSettings": string;
  "command.toggleTheme": string;
  "command.categoryActions": string;
  "command.categoryTables": string;
  "command.narrowSearch": string;

  // ====== 数据库切换器 ======
  "dbSwitcher.searchPlaceholder": string;
  "dbSwitcher.noMatch": string;

  // ====== 状态栏 ======
  "statusBar.tables": string;
  "statusBar.rows": string;
  "statusBar.switchDatabase": string;

  // ====== 日志查看器 ======
  "logViewer.title": string;
  "logViewer.refresh": string;
  "logViewer.noLogs": string;
  "logViewer.loadFailed": string;
  "logViewer.loading": string;

  // ====== 文档标签 ======
  "doc.prefix": string;

  // ====== 导出标签 ======
  "export.prefix": string;

  // ====== 首页空状态 ======
  "empty.title": string;
  "empty.subtitle": string;
  "empty.quickSearch": string;
  "empty.newConnection": string;
  "empty.newQuery": string;

  // ====== Tab 溢出菜单 ======
  "tabs.moreTabs": string;
}

export type Locale = "zh-CN" | "en-US";
export type TranslationKey = keyof TranslationKeys;
