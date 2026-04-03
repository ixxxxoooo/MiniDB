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
  "common.stop": string;
  "common.done": string;
  "common.cancelled": string;
  "common.rows": string;
  "window.minimize": string;
  "window.maximize": string;

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
  "toolbar.reconnect": string;

  // ====== 侧边栏 ======
  "sidebar.noConnections": string;
  "sidebar.addConnection": string;
  "sidebar.notConnected": string;
  "sidebar.connect": string;
  "sidebar.disconnect": string;
  "sidebar.editConnection": string;
  "sidebar.tables": string;
  "sidebar.searchPlaceholder": string;
  "sidebar.noTables": string;
  "sidebar.connectHint": string;

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
  "connection.recentConnections": string;
  "connection.recentConnectionsDesc": string;
  "connection.chooseDriver": string;
  "connection.driverHint": string;
  "connection.savedLocally": string;
  "connection.noDefaultDatabase": string;
  "connection.connectionDetails": string;
  "connection.namePlaceholder": string;
  "connection.hostPlaceholder": string;
  "connection.userPlaceholder": string;
  "connection.passwordPlaceholder": string;
  "connection.databasePlaceholder": string;
  "connection.databaseFilePlaceholder": string;
  "connection.tag": string;
  "connection.tagLocal": string;
  "connection.tagTest": string;
  "connection.tagProduction": string;
  "connection.requiredField": string;
  "connection.driverConnectionTitle": string;
  "connection.color": string;

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
  "datagrid.anyColumn": string;
  "datagrid.rawSQLExample": string;

  // ====== 右键菜单（行） ======
  "contextMenu.previewRow": string;
  "contextMenu.copyCell": string;
  "contextMenu.formatJSON": string;
  "contextMenu.copyRow": string;
  "contextMenu.copyAsInsert": string;
  "contextMenu.downloadCSV": string;
  "contextMenu.deleteRow": string;

  // ====== JSON 预览 ======
  "jsonViewer.title": string;
  "jsonViewer.copyJSON": string;

  // ====== 右键菜单（表） ======
  "contextMenu.openInNewTab": string;
  "contextMenu.copyTableName": string;
  "contextMenu.viewData": string;
  "contextMenu.viewStructure": string;
  "contextMenu.viewDDL": string;
  "contextMenu.tableDoc": string;
  "contextMenu.exportData": string;
  "contextMenu.exportCSV": string;
  "contextMenu.exportJSON": string;
  "contextMenu.exportSQL": string;
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
  "editor.history": string;
  "editor.favorites": string;
  "editor.searchHistory": string;
  "editor.searchFavorites": string;
  "editor.clearHistory": string;
  "editor.noHistory": string;
  "editor.noFavorites": string;
  "editor.favorite": string;
  "editor.unfavorite": string;
  "editor.aiNeedInput": string;
  "editor.aiGenerateEmpty": string;
  "editor.aiGenerateDone": string;
  "editor.aiCheckDone": string;
  "editor.aiCheckFormatError": string;
  "editor.aiFailed": string;
  "editor.aiHint": string;
  "editor.aiPreview": string;
  "editor.aiPreviewFixTitle": string;
  "editor.aiPreviewGenTitle": string;
  "editor.aiPreviewFixDesc": string;
  "editor.aiPreviewGenDesc": string;
  "editor.aiChanged": string;
  "editor.aiUnchanged": string;
  "editor.aiCopyResult": string;
  "editor.aiApplyToEditor": string;
  "editor.aiExplanation": string;
  "editor.aiInputText": string;
  "editor.aiGeneratedSQL": string;
  "editor.aiCheckPassed": string;
  "editor.aiCheckFixed": string;
  "editor.aiSelection": string;
  "editor.aiFullText": string;
  "editor.aiFromSelection": string;
  "editor.aiFromFullText": string;
  "editor.resultPlaceholder": string;
  "editor.shortcutsHint": string;

  // ====== 设置 ======
  "settings.title": string;
  "settings.general": string;
  "settings.aiConfig": string;
  "settings.appearance": string;
  "settings.language": string;
  "settings.about": string;

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
  "generalSettings.showScrollbar": string;
  "generalSettings.showScrollbarDesc": string;
  "generalSettings.shortcutsTitle": string;
  "generalSettings.shortcutsDescription": string;
  "generalSettings.shortcutsGlobal": string;
  "generalSettings.shortcutsTableView": string;
  "generalSettings.shortcutsSqlEditor": string;
  "generalSettings.shortcutsAiPanel": string;
  "generalSettings.shortcutsDocEditor": string;
  "generalSettings.shortcutsDialog": string;
  "about.title": string;
  "about.description": string;
  "about.appName": string;
  "about.version": string;
  "about.author": string;
  "about.email": string;
  "about.license": string;
  "about.techStack": string;
  "about.summary": string;

  // ====== AI 设置 ======
  "aiSettings.title": string;
  "aiSettings.description": string;
  "aiSettings.baseURL": string;
  "aiSettings.apiKey": string;
  "aiSettings.model": string;
  "aiSettings.systemPrompt": string;
  "aiSettings.systemPromptHint": string;
  "aiSettings.systemPromptPlaceholder": string;
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
  "aiSettings.baseURLPlaceholder": string;
  "aiSettings.apiKeyPlaceholder": string;
  "aiSettings.modelPlaceholder": string;

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
  "ai.retry": string;
  "ai.autoFixing": string;
  "ai.autoFixAttempt": string;
  "ai.autoFixFailed": string;
  "ai.autoFixSuccess": string;
  "ai.fixWithAI": string;
  "ai.sqlErrorFeedback": string;
  "ai.tokenCount": string;
  "ai.charCount": string;
  "ai.answerAt": string;
  "ai.duration": string;
  "ai.statusLoadingSchema": string;
  "ai.statusPlanningTools": string;
  "ai.statusCallingAI": string;
  "ai.statusExecutingSQL": string;
  "ai.statusAutoFixing": string;
  "ai.statusDone": string;
  "ai.autoExecuteSkippedUnsafe": string;
  "ai.autoExecuteSkippedUnsafeReason": string;
  "ai.viewProcess": string;
  "ai.hideProcess": string;
  "ai.progressDone": string;
  "ai.mermaidPreviewLabel": string;
  "ai.mermaidRenderFailed": string;
  "ai.toolTimelineTitle": string;
  "ai.toolUnknown": string;
  "ai.toolEventStart": string;
  "ai.toolEventSQL": string;
  "ai.toolEventResult": string;
  "ai.toolEventError": string;
  "ai.mentionTableTitle": string;
  "ai.mentionToolTitle": string;
  "ai.mentionTableHint": string;
  "ai.applyAndExecute": string;

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
  "logViewer.exportTitle": string;
  "logViewer.exportInProgress": string;
  "logViewer.exportDone": string;
  "logViewer.exportFailed": string;
  "logViewer.exportCancelled": string;

  // ====== Markdown / 文档编辑器 ======
  "markdown.placeholder": string;
  "markdown.enterLink": string;
  "markdown.bold": string;
  "markdown.italic": string;
  "markdown.strike": string;
  "markdown.inlineCode": string;
  "markdown.highlight": string;
  "markdown.heading1": string;
  "markdown.heading2": string;
  "markdown.heading3": string;
  "markdown.bulletList": string;
  "markdown.orderedList": string;
  "markdown.blockquote": string;
  "markdown.codeBlock": string;
  "markdown.horizontalRule": string;
  "markdown.insertLink": string;
  "markdown.undo": string;
  "markdown.redo": string;
  "rowPreview.copyJSON": string;
  "rowPreview.copyInsert": string;

  // ====== 结构编辑 / 查询结果 ======
  "structure.columns": string;
  "structure.addColumn": string;
  "structure.deleteSelectedColumn": string;
  "structure.revertAll": string;
  "structure.noMatchingTypes": string;
  "structure.noIndexesFound": string;
  "structure.addIndex": string;
  "structure.indexName": string;
  "structure.columnNames": string;
  "structure.unique": string;
  "structure.uniqueIndex": string;
  "structure.indexNamePlaceholder": string;
  "structure.indexColumnsPlaceholder": string;
  "structure.indexInlineRequired": string;
  "query.executionFailed": string;
  "query.pageFailed": string;
  "query.showResultHint": string;
  "query.empty": string;
  "query.null": string;
  "structure.commitFailed": string;
  "structure.operationFailed": string;
  "structure.noColumnsFound": string;
  "structure.dropIndexConfirm": string;
  "ai.emptySQLReason": string;
  "ai.riskySQLReason": string;
  "ai.unknownSQLReason": string;

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
