import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Check,
  CircleCheck,
  CircleX,
  Clock3,
  Database,
  FolderOpen,
  MessageSquare,
  RefreshCcw,
  Search,
  SendHorizontal,
  Settings,
  Square,
  Upload,
  X,
} from 'lucide-react';
import {useEffect, useMemo, useState} from 'react';

type TopicRef = {id: number; title: string};
type ChatRef = {id: number; username: string; title: string; type: string; topics: TopicRef[]; self: boolean};
type ChatPage = {items: ChatRef[]; next: number; skipped: number};
type DesktopSettings = {schemaVersion: number; namespace: string; proxy: string; threads: number; limit: number; downloadDirectory: string; coverMode: string; coverAt: string; downloadGroup: boolean; downloadSkipSame: boolean; downloadRewrite: boolean};
type MediaPreview = {dataURL: string; kind: string; path: string; name: string; width: number; height: number};
type TaskRecord = {id: number; type: string; detail: string; status: string; summary: string; error: string; startedAt: string; finishedAt: string};
type TransferItem = {
  TaskID: string;
  FileName: string;
  SourcePath: string;
  CompletedBytes: number;
  TotalBytes: number;
  Status: string;
  Phase: string;
  Err: string;
  At: string;
};
type Snapshot = {
  Pending: number;
  Running: number;
  Succeeded: number;
  Failed: number;
  Canceled: number;
  Skipped: number;
  CompletedBytes: number;
  TotalBytes: number;
  Speed: number;
  CurrentFile: string;
  CurrentSourcePath: string;
  DiscoveryDone: boolean;
  Final: boolean;
  Unknown: number;
  Status: string;
  Errors: string[];
};

const emptySnapshot: Snapshot = {
  Pending: 0,
  Running: 0,
  Succeeded: 0,
  Failed: 0,
  Canceled: 0,
  Skipped: 0,
  CompletedBytes: 0,
  TotalBytes: 0,
  Speed: 0,
  CurrentFile: '',
  CurrentSourcePath: '',
  DiscoveryDone: false,
  Final: false,
  Unknown: 0,
  Status: '',
  Errors: [],
};

const savedMessages: ChatRef = {id: 0, username: '', title: 'Saved Messages', type: 'self', topics: [], self: true};
const defaultSettings: DesktopSettings = {schemaVersion: 1, namespace: 'default', proxy: '', threads: 4, limit: 2, downloadDirectory: 'downloads', coverMode: 'video-cover', coverAt: 'auto', downloadGroup: true, downloadSkipSame: true, downloadRewrite: false};
type WorkspacePage = 'upload' | 'download' | 'forward' | 'chat' | 'tasks' | 'backup' | 'recover' | 'settings';
const nav = [
  [Upload, '上传', 'upload'],
  [ArrowDownToLine, '下载', 'download'],
  [SendHorizontal, '转发', 'forward'],
  [MessageSquare, '会话', 'chat'],
  [ArrowUpFromLine, '任务', 'tasks'],
  [Database, '备份', 'backup'],
  [RefreshCcw, '恢复', 'recover'],
  [Settings, '设置', 'settings'],
] as const;

export function App() {
  const [paths, setPaths] = useState<string[]>([]);
  const [preview, setPreview] = useState<MediaPreview | null>(null);
  const [previewError, setPreviewError] = useState('');
  const [activePage, setActivePage] = useState<WorkspacePage>('upload');
  const [downloadInput, setDownloadInput] = useState('');
  const [downloadFiles, setDownloadFiles] = useState<string[]>([]);
  const [downloadDirectory, setDownloadDirectory] = useState('downloads');
  const [downloadGroup, setDownloadGroup] = useState(true);
  const [downloadSkipSame, setDownloadSkipSame] = useState(true);
  const [downloadRewrite, setDownloadRewrite] = useState(false);
  const [settings, setSettings] = useState<DesktopSettings>(defaultSettings);
  const [settingsDraft, setSettingsDraft] = useState<DesktopSettings>(defaultSettings);
  const [settingsStatus, setSettingsStatus] = useState('');
  const [tasks, setTasks] = useState<TaskRecord[]>([]);
  const [operationStatus, setOperationStatus] = useState('');
  const [forwardInput, setForwardInput] = useState('');
  const [forwardFiles, setForwardFiles] = useState<string[]>([]);
  const [forwardTo, setForwardTo] = useState('');
  const [forwardMode, setForwardMode] = useState('direct');
  const [forwardSilent, setForwardSilent] = useState(false);
  const [forwardDryRun, setForwardDryRun] = useState(false);
  const [chatExportLast, setChatExportLast] = useState(100);
  const [chatExportOutput, setChatExportOutput] = useState('');
  const [chatExportContent, setChatExportContent] = useState(false);
  const [backupPath, setBackupPath] = useState('');
  const [recoveryPath, setRecoveryPath] = useState('');
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [namespace, setNamespace] = useState('default');
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [snap, setSnap] = useState<Snapshot>(emptySnapshot);
  const [items, setItems] = useState<TransferItem[]>([]);
  const [chatOpen, setChatOpen] = useState(false);
  const [chatItems, setChatItems] = useState<ChatRef[]>([]);
  const [chatNext, setChatNext] = useState(0);
  const [chatLoading, setChatLoading] = useState(false);
  const [chatError, setChatError] = useState('');
  const [chatQuery, setChatQuery] = useState('');
  const [selectedChat, setSelectedChat] = useState<ChatRef>(savedMessages);
  const [selectedTopic, setSelectedTopic] = useState<TopicRef | null>(null);
  const [draftChat, setDraftChat] = useState<ChatRef>(savedMessages);
  const [draftTopic, setDraftTopic] = useState<TopicRef | null>(null);

  useEffect(() => {
    Promise.allSettled([window.go?.main.App.Namespaces(), window.go?.main.App.GetSettings()]).then(([namespaceResult, settingsResult]) => {
      const values = namespaceResult.status === 'fulfilled' ? namespaceResult.value : [];
      setNamespaces(values);
      if (namespaceResult.status === 'rejected') setError(errorText(namespaceResult.reason));
      if (settingsResult.status === 'fulfilled') {
        const loaded = settingsResult.value;
        setSettings(loaded);
        setSettingsDraft(loaded);
        setDownloadDirectory(loaded.downloadDirectory);
        setDownloadGroup(loaded.downloadGroup);
        setDownloadSkipSame(loaded.downloadSkipSame);
        setDownloadRewrite(loaded.downloadRewrite);
        setNamespace(values.includes(loaded.namespace) ? loaded.namespace : values[0] || loaded.namespace);
      } else {
        setError(errorText(settingsResult.reason));
        if (values.length) setNamespace(values.includes('default') ? 'default' : values[0]);
      }
    });
    const offSnapshot = window.runtime?.EventsOn('transfer:snapshot', (value: Snapshot) => setSnap(value));
    const offItems = window.runtime?.EventsOn('transfer:items', (value: TransferItem[]) => setItems(value || []));
    const offDone = window.runtime?.EventsOn('transfer:done', (value: {error?: string}) => {
      setRunning(false);
      setError(value.error || '');
      setOperationStatus(value.error ? `操作失败：${value.error}` : '操作已完成');
    });
    const offTasks = window.runtime?.EventsOn('tasks:changed', (value: TaskRecord[]) => setTasks(value || []));
    window.go?.main.App.TaskHistory().then(setTasks).catch(() => undefined);
    return () => {
      offSnapshot?.();
      offItems?.();
      offDone?.();
      offTasks?.();
    };
  }, []);

  useEffect(() => {
    setSelectedChat(savedMessages);
    setSelectedTopic(null);
    setChatItems([]);
    setChatNext(0);
    setChatError('');
  }, [namespace]);

  useEffect(() => {
    if (activePage === 'upload' && snap.CurrentSourcePath && snap.CurrentSourcePath !== preview?.path) {
      loadMediaPreview([snap.CurrentSourcePath]);
    }
  }, [activePage, snap.CurrentSourcePath]);

  const knownTotal = snap.DiscoveryDone && snap.Unknown === 0 && snap.TotalBytes > 0;
  const pct = knownTotal ? Math.min(100, (snap.CompletedBytes * 100) / snap.TotalBytes) : 0;
  const eta = knownTotal && snap.Speed > 0 && !snap.Final ? duration((snap.TotalBytes - snap.CompletedBytes) / snap.Speed) : '--';
  const targetLabel = selectedTopic ? `${selectedChat.title} · ${selectedTopic.title}` : selectedChat.title;
  const filteredChats = useMemo(() => {
    const query = chatQuery.trim().toLocaleLowerCase();
    if (!query) return chatItems;
    return chatItems.filter((entry) => `${entry.title} ${entry.username} ${entry.type}`.toLocaleLowerCase().includes(query));
  }, [chatItems, chatQuery]);
  const downloadURLs = useMemo(() => downloadInput.split(/\s+/).map((value) => value.trim()).filter(Boolean), [downloadInput]);
  const forwardSources = useMemo(() => [...forwardInput.split(/\s+/).map((value) => value.trim()).filter(Boolean), ...forwardFiles], [forwardInput, forwardFiles]);
  const settingsDirty = JSON.stringify(settingsDraft) !== JSON.stringify(settings);

  function switchPage(page: WorkspacePage) {
    if (running || page === activePage) return;
    if (activePage === 'settings' && settingsDirty && !window.confirm('设置尚未应用，确定放弃修改吗？')) return;
    if (page === 'settings') {
      setSettingsDraft(settings);
      setSettingsStatus('');
    }
    if ((page === 'chat' || page === 'forward') && !chatItems.length && !chatLoading) void loadChats(0, true);
    if (page === 'tasks') void window.go.main.App.TaskHistory().then(setTasks);
    setActivePage(page);
    setError('');
    setSnap(emptySnapshot);
    setItems([]);
    setOperationStatus('');
  }

  async function pickFiles() {
    const value = await window.go.main.App.SelectUploadFiles();
    if (value?.length) {
      setPaths(value);
      await loadMediaPreview(value);
    }
  }

  async function pickDirectory() {
    const value = await window.go.main.App.SelectUploadDirectory();
    if (value) {
      setPaths([value]);
      await loadMediaPreview([value]);
    }
  }

  async function loadMediaPreview(values: string[]) {
    setPreviewError('');
    try {
      setPreview(await window.go.main.App.MediaPreview(values));
    } catch (reason) {
      setPreview(null);
      setPreviewError(errorText(reason));
    }
  }

  async function pickDownloadDirectory() {
    const value = await window.go.main.App.SelectDownloadDirectory();
    if (value) setDownloadDirectory(value);
  }

  async function pickDownloadFiles() {
    const value = await window.go.main.App.SelectDownloadExportFiles();
    if (value?.length) setDownloadFiles(value);
  }

  async function pickForwardFiles() {
    const value = await window.go.main.App.SelectForwardFiles();
    if (value?.length) setForwardFiles(value);
  }

  async function pickChatExportDestination() {
    const value = await window.go.main.App.SelectChatExportDestination();
    if (value) setChatExportOutput(value);
  }

  async function pickBackupDestination() {
    const value = await window.go.main.App.SelectBackupDestination();
    if (value) setBackupPath(value);
  }

  async function pickRecoveryFile() {
    const value = await window.go.main.App.SelectRecoveryFile();
    if (value) setRecoveryPath(value);
  }

  async function pickSettingsDownloadDirectory() {
    const value = await window.go.main.App.SelectDownloadDirectory();
    if (value) setSettingsDraft((current) => ({...current, downloadDirectory: value}));
  }

  async function loadChats(cursor = 0, reset = false) {
    setChatLoading(true);
    setChatError('');
    try {
      const page: ChatPage = await window.go.main.App.ChatPage(namespace, cursor, 40);
      setChatItems((previous) => uniqueChats(reset ? page.items : [...previous, ...page.items]));
      setChatNext(page.next || 0);
    } catch (reason) {
      setChatError(errorText(reason));
    } finally {
      setChatLoading(false);
    }
  }

  async function openChatSelector() {
    setDraftChat(selectedChat);
    setDraftTopic(selectedTopic);
    setChatOpen(true);
    if (!chatItems.length) await loadChats(0, true);
  }

  function confirmChatSelector() {
    setSelectedChat(draftChat);
    setSelectedTopic(draftTopic);
    setChatOpen(false);
  }

  async function startUpload() {
    setError('');
    setSnap(emptySnapshot);
    setItems([]);
    try {
      await window.go.main.App.StartUpload({
        namespace,
        proxy: settings.proxy,
        paths,
        chat: selectedChat.self ? '' : String(selectedChat.id),
        topic: selectedTopic?.id || 0,
        coverMode: settings.coverMode,
        coverAt: settings.coverAt,
        threads: settings.threads,
        limit: settings.limit,
      });
      setRunning(true);
    } catch (reason) {
      setError(errorText(reason));
    }
  }

  async function startDownload() {
    setError('');
    setSnap(emptySnapshot);
    setItems([]);
    try {
      await window.go.main.App.StartDownload({
        namespace,
        proxy: settings.proxy,
        urls: downloadURLs,
        files: downloadFiles,
        directory: downloadDirectory,
        threads: settings.threads,
        limit: settings.limit,
        group: downloadGroup,
        skipSame: downloadSkipSame,
        rewrite: downloadRewrite,
      });
      setRunning(true);
    } catch (reason) {
      setError(errorText(reason));
    }
  }

  async function saveSettings() {
    setSettingsStatus('');
    try {
      const saved = await window.go.main.App.SaveSettings(settingsDraft);
      setSettings(saved);
      setSettingsDraft(saved);
      setDownloadDirectory(saved.downloadDirectory);
      setDownloadGroup(saved.downloadGroup);
      setDownloadSkipSame(saved.downloadSkipSame);
      setDownloadRewrite(saved.downloadRewrite);
      if (namespaces.includes(saved.namespace)) setNamespace(saved.namespace);
      setSettingsStatus('设置已保存并应用');
    } catch (reason) {
      setSettingsStatus(errorText(reason));
    }
  }

  async function startForward() {
    setError('');
    setOperationStatus('');
    try {
      await window.go.main.App.StartForward({namespace, proxy: settings.proxy, from: forwardSources, to: forwardTo, mode: forwardMode, threads: settings.threads, silent: forwardSilent, dryRun: forwardDryRun});
      setRunning(true);
      setOperationStatus('转发任务运行中');
    } catch (reason) {
      setOperationStatus(errorText(reason));
    }
  }

  async function startChatExport() {
    setOperationStatus('');
    try {
      await window.go.main.App.StartChatExport({namespace, proxy: settings.proxy, chat: selectedChat.self ? '' : String(selectedChat.id), topic: selectedTopic?.id || 0, last: chatExportLast, output: chatExportOutput, withContent: chatExportContent});
      setRunning(true);
      setOperationStatus('会话导出运行中');
    } catch (reason) {
      setOperationStatus(errorText(reason));
    }
  }

  async function startBackup() {
    setOperationStatus('');
    try {
      await window.go.main.App.StartBackup(backupPath);
      setRunning(true);
      setOperationStatus('备份运行中');
    } catch (reason) {
      setOperationStatus(errorText(reason));
    }
  }

  async function startRecovery() {
    if (!window.confirm('恢复会覆盖同名账号数据。确定使用此备份继续吗？')) return;
    setOperationStatus('');
    try {
      await window.go.main.App.StartRecover(recoveryPath, true);
      setRunning(true);
      setOperationStatus('恢复运行中');
    } catch (reason) {
      setOperationStatus(errorText(reason));
    }
  }

  return (
    <main className="shell">
      <aside className="sidebar">
        <section className="brand"><strong>TMT</strong><span>Telegram Media Transfer</span></section>
        <nav>{nav.map(([Icon, label, page]) => <button disabled={!page || running} title={!page ? '后续阶段实现' : ''} className={page === activePage ? 'active' : ''} key={label} onClick={() => page && switchPage(page)}><Icon size={21}/><span>{label}</span></button>)}</nav>
      </aside>

      <section className="workspace">
        <header className="session">
          <span>工作区：<b>{pageLabel(activePage)}</b></span><i/>
          <span>账号：<select disabled={running || chatLoading || activePage === 'settings'} value={namespace} onChange={(event) => setNamespace(event.target.value)}>{namespaces.map((value) => <option key={value}>{value}</option>)}</select></span><i/>
          <span>状态：<mark>● {running ? '传输中' : '就绪'}</mark></span>
          <time>{new Date().toLocaleString()}</time>
        </header>

        {activePage === 'settings' ? <article className="settings-page card">
          <header><div><small>DESKTOP PREFERENCES</small><h1>全局设置</h1><p>这些值会应用到之后启动的上传和下载任务。保存失败时，当前生效值不会改变。</p></div><Settings size={34}/></header>
          <div className="settings-grid">
            <label><span>默认账号<small>新任务默认使用的 Telegram 命名空间</small></span><select value={settingsDraft.namespace} onChange={(event) => setSettingsDraft({...settingsDraft, namespace: event.target.value})}>{namespaces.map((value) => <option key={value}>{value}</option>)}</select></label>
            <label><span>代理地址<small>留空时直接连接，例如 socks5://127.0.0.1:1080</small></span><input value={settingsDraft.proxy} onChange={(event) => setSettingsDraft({...settingsDraft, proxy: event.target.value})} placeholder="protocol://host:port"/></label>
            <label><span>单文件线程数<small>一个文件内部的并行传输线程，范围 1–64</small></span><input type="number" min="1" max="64" value={settingsDraft.threads} onChange={(event) => setSettingsDraft({...settingsDraft, threads: Number(event.target.value)})}/></label>
            <label><span>并行任务数<small>同时处理的文件数量，范围 1–32</small></span><input type="number" min="1" max="32" value={settingsDraft.limit} onChange={(event) => setSettingsDraft({...settingsDraft, limit: Number(event.target.value)})}/></label>
            <label><span>视频封面模式<small>video_cover 高清封面不会改变视频起播时间</small></span><select value={settingsDraft.coverMode} onChange={(event) => setSettingsDraft({...settingsDraft, coverMode: event.target.value})}><option value="video-cover">video_cover · 高清</option><option value="thumbnail">thumbnail · 快速</option><option value="off">不生成封面</option></select></label>
            <label><span>封面取帧时间<small>auto 自动选择，也可输入 2s、00:00:05</small></span><input value={settingsDraft.coverAt} onChange={(event) => setSettingsDraft({...settingsDraft, coverAt: event.target.value})} placeholder="auto"/></label>
            <label className="wide"><span>默认下载目录<small>下载页首次打开和应用设置后使用的目录</small></span><div className="path-setting"><input readOnly value={settingsDraft.downloadDirectory}/><button onClick={pickSettingsDownloadDirectory}><FolderOpen size={16}/>选择</button></div></label>
            <label className="wide"><span>下载默认行为<small>每次仍可在下载页单独修改</small></span><div className="setting-checks"><label><input type="checkbox" checked={settingsDraft.downloadGroup} onChange={(event) => setSettingsDraft({...settingsDraft, downloadGroup: event.target.checked})}/> 自动下载媒体组</label><label><input type="checkbox" checked={settingsDraft.downloadSkipSame} onChange={(event) => setSettingsDraft({...settingsDraft, downloadSkipSame: event.target.checked})}/> 跳过同名同大小</label><label><input type="checkbox" checked={settingsDraft.downloadRewrite} onChange={(event) => setSettingsDraft({...settingsDraft, downloadRewrite: event.target.checked})}/> 修正扩展名</label></div></label>
          </div>
          <footer><span className={settingsStatus === '设置已保存并应用' ? 'settings-ok' : 'settings-error'}>{settingsStatus || (settingsDirty ? '有尚未应用的修改' : '设置已与磁盘同步')}</span><div><button disabled={!settingsDirty} onClick={() => {setSettingsDraft(settings); setSettingsStatus('已放弃未应用的修改')}}>取消修改</button><button className="apply" disabled={!settingsDirty} onClick={saveSettings}>应用设置</button></div></footer>
        </article> : activePage === 'forward' ? <article className="tool-page card">
          <header><div><small>MESSAGE ROUTER</small><h1>转发消息</h1><p>输入消息链接或选择会话导出文件，然后指定目标会话。</p></div><SendHorizontal size={34}/></header>
          <div className="tool-grid">
            <section><label>来源</label><textarea value={forwardInput} onChange={(event) => setForwardInput(event.target.value)} placeholder={'Telegram 消息链接，每行一个\n也可以导入会话导出的 JSON 文件'}/><div className="inline-actions"><button onClick={pickForwardFiles}><FolderOpen size={16}/>导入 JSON</button><span>{forwardFiles.length ? `已导入 ${forwardFiles.length} 个文件` : `${forwardSources.length} 个来源`}</span></div></section>
            <section><label>目标与模式</label><select value={forwardTo} onChange={(event) => setForwardTo(event.target.value)}><option value="">Saved Messages</option>{chatItems.filter((entry) => !entry.self).map((entry) => <option key={chatKey(entry)} value={String(entry.id)}>{entry.title || entry.username}</option>)}</select><select value={forwardMode} onChange={(event) => setForwardMode(event.target.value)}><option value="direct">直接转发</option><option value="clone">复制并重新上传</option></select><div className="setting-checks"><label><input type="checkbox" checked={forwardSilent} onChange={(event) => setForwardSilent(event.target.checked)}/> 静默发送</label><label><input type="checkbox" checked={forwardDryRun} onChange={(event) => setForwardDryRun(event.target.checked)}/> 仅预演</label></div></section>
          </div>
          <footer><span>{operationStatus || (chatLoading ? '正在加载目标会话…' : '准备就绪')}</span><div>{running && <button className="danger" onClick={() => window.go.main.App.StopTransfer()}><Square size={14}/>停止</button>}<button className="apply" disabled={!forwardSources.length || running} onClick={startForward}>开始转发</button></div></footer>
        </article> : activePage === 'chat' ? <article className="tool-page chat-workspace card">
          <header><div><small>TELEGRAM CHATS</small><h1>会话与导出</h1><p>浏览当前账号的会话和话题，并将最近消息导出为下载或转发可用的 JSON。</p></div><MessageSquare size={34}/></header>
          <div className="chat-tool-body">
            <section className="chat-browser"><div className="chat-search"><Search size={17}/><input placeholder="搜索会话" value={chatQuery} onChange={(event) => setChatQuery(event.target.value)}/></div><div className="chat-tool-list">{filteredChats.map((entry) => <button className={sameChat(entry, selectedChat) ? 'selected' : ''} key={chatKey(entry)} onClick={() => {setSelectedChat(entry);setSelectedTopic(null)}}><span className="chat-avatar">{entry.self ? '★' : (entry.title || '?').slice(0, 1)}</span><span><b>{entry.title}</b><small>{entry.self ? '个人收藏夹' : typeLabel(entry.type)}</small></span></button>)}</div>{chatNext > 0 && <button className="load-more" disabled={chatLoading} onClick={() => loadChats(chatNext)}>{chatLoading ? '加载中…' : '加载更多'}</button>}</section>
            <section className="export-panel"><small>当前会话</small><h2>{selectedChat.title}</h2>{selectedChat.topics.length > 0 && <select value={selectedTopic?.id || 0} onChange={(event) => setSelectedTopic(selectedChat.topics.find((topic) => topic.id === Number(event.target.value)) || null)}><option value="0">不指定话题</option>{selectedChat.topics.map((topic) => <option key={topic.id} value={topic.id}>{topic.title}</option>)}</select>}<label>最近消息数<input type="number" min="1" max="100000" value={chatExportLast} onChange={(event) => setChatExportLast(Number(event.target.value))}/></label><label>导出文件<div className="path-setting"><input readOnly value={chatExportOutput}/><button onClick={pickChatExportDestination}><FolderOpen size={16}/>选择</button></div></label><label className="check-line"><input type="checkbox" checked={chatExportContent} onChange={(event) => setChatExportContent(event.target.checked)}/>包含消息正文</label><button className="apply" disabled={!chatExportOutput || running} onClick={startChatExport}>导出最近消息</button><p>{operationStatus}</p></section>
          </div>
        </article> : activePage === 'tasks' ? <article className="tool-page card">
          <header><div><small>RUN HISTORY</small><h1>任务记录</h1><p>显示本次 TMT 运行期间执行的上传、下载、转发、导出、备份和恢复。</p></div><ArrowUpFromLine size={34}/></header>
          <div className="task-history"><div className="task-history-head"><span>类型</span><span>状态</span><span>开始时间</span><span>详情</span></div>{!tasks.length && <div className="tool-empty">尚无任务记录</div>}{tasks.map((task) => <div className="task-history-row" key={task.id}><b>{operationLabel(task.type)}</b><span className={`state-${task.status}`}>{statusLabel(task.status)}</span><time>{new Date(task.startedAt).toLocaleString()}</time><span>{task.error || task.summary || task.detail}</span></div>)}</div>
          <footer><span>共 {tasks.length} 条记录</span><div><button disabled={!tasks.length || running} onClick={async () => {await window.go.main.App.ClearTaskHistory();setTasks([])}}>清空记录</button></div></footer>
        </article> : activePage === 'backup' ? <article className="tool-page compact-tool card">
          <header><div><small>ACCOUNT BACKUP</small><h1>备份账号数据</h1><p>将当前兼容数据目录中的账号会话与元数据压缩为一个 TMT 备份文件。</p></div><Database size={34}/></header>
          <div className="single-operation"><Database size={52}/><h2>创建备份</h2><div className="path-setting"><input readOnly value={backupPath} placeholder="选择 .tmt 备份保存位置"/><button onClick={pickBackupDestination}><FolderOpen size={16}/>选择</button></div><button className="apply" disabled={!backupPath || running} onClick={startBackup}>开始备份</button><p>{operationStatus}</p></div>
        </article> : activePage === 'recover' ? <article className="tool-page compact-tool card">
          <header><div><small>ACCOUNT RECOVERY</small><h1>恢复账号数据</h1><p>支持新的 .tmt 备份和旧版 .tdl 备份。执行前会再次确认覆盖风险。</p></div><RefreshCcw size={34}/></header>
          <div className="single-operation warning"><RefreshCcw size={52}/><h2>选择备份文件</h2><div className="path-setting"><input readOnly value={recoveryPath} placeholder="选择 .tmt 或 .tdl 文件"/><button onClick={pickRecoveryFile}><FolderOpen size={16}/>选择</button></div><button className="apply" disabled={!recoveryPath || running} onClick={startRecovery}>确认并恢复</button><p>{operationStatus}</p></div>
        </article> : <>
        <div className="overview">
          <article className="media card">
            <label>{activePage === 'upload' ? 'CURRENT MEDIA' : 'DOWNLOAD SOURCE'}</label>
            {activePage === 'upload'
              ? <div className={`cover preview-cover ${preview ? 'has-preview' : ''}`}>
                  {preview ? <><img src={preview.dataURL} alt={preview.name}/><span className="preview-kind">{preview.kind === 'video' ? 'VIDEO' : 'IMAGE'}</span><small>{preview.name}<br/>{preview.width}×{preview.height}</small></> : <><FolderOpen size={34}/><span>MEDIA</span><small>{previewError || '选择图片或视频后显示预览'}</small></>}
                </div>
              : <div className="cover download-cover"><ArrowDownToLine size={32}/><span>LINKS</span><small>{downloadURLs.length} 个链接 · {downloadFiles.length} 个导出文件</small></div>}
          </article>

          <article className="task card">
            <h2>{activePage === 'upload' ? <Upload/> : <ArrowDownToLine/>} 当前任务 <span>· {activePage === 'upload' ? '上传' : '下载'}</span></h2><hr/>
            {activePage === 'upload' ? <>
              <h1>{snap.CurrentFile || paths[0]?.split(/[\\/]/).pop() || '选择要上传的文件'}</h1>
              <p>{paths.length ? `已选择 ${paths.length} 项` : '支持文件和目录 · 视频默认生成高清封面'}</p>
              <div className="target-row"><span>发送到</span><button disabled={running} onClick={openChatSelector}><MessageSquare size={15}/>{targetLabel}</button></div>
              <div className="picker"><button disabled={running} onClick={pickFiles}>选择文件</button><button disabled={running} onClick={pickDirectory}>选择目录</button><button className="primary" disabled={!paths.length || running} onClick={startUpload}>开始上传</button></div>
            </> : <>
              <textarea className="download-input" disabled={running} value={downloadInput} onChange={(event) => setDownloadInput(event.target.value)} placeholder={'粘贴 Telegram 消息链接，每行一个\n例如：https://t.me/c/123456/789'}/>
              <div className="download-destination"><span>保存到</span><button disabled={running} onClick={pickDownloadDirectory}><FolderOpen size={15}/><b>{downloadDirectory}</b></button><button disabled={running} onClick={pickDownloadFiles}>导入 JSON</button></div>
              <div className="download-options"><label><input type="checkbox" checked={downloadGroup} onChange={(event) => setDownloadGroup(event.target.checked)}/> 自动下载媒体组</label><label><input type="checkbox" checked={downloadSkipSame} onChange={(event) => setDownloadSkipSame(event.target.checked)}/> 跳过同名同大小</label><label><input type="checkbox" checked={downloadRewrite} onChange={(event) => setDownloadRewrite(event.target.checked)}/> 修正扩展名</label><button className="primary" disabled={(!downloadURLs.length && !downloadFiles.length) || running} onClick={startDownload}>开始下载</button></div>
            </>}
            <div className={`progress ${knownTotal ? '' : 'indeterminate'}`}><span style={knownTotal ? {width: `${pct}%`} : undefined}/><b>{knownTotal ? `${pct.toFixed(1)}%` : '--'}</b></div>
            <p>{activePage === 'upload' ? '已上传' : '已下载'}&nbsp; <strong>{bytes(snap.CompletedBytes)} / {knownTotal ? bytes(snap.TotalBytes) : '--'}</strong></p>
            <div className="metrics">
              <span>◴ 速度&nbsp; <b>{bytes(snap.Speed)}/s</b>&nbsp;&nbsp; ETA&nbsp; <b>{eta}</b></span>
              <button className="danger" disabled={!running} onClick={() => window.go.main.App.StopTransfer()}><Square size={13}/>停止</button>
            </div>
            {error && <p className="error">{error}</p>}
          </article>

          <article className="stats card"><h2>任务统计</h2><hr/>
            <Stat icon={<Clock3/>} name="待处理" value={String(snap.Pending || 0)}/>
            <Stat icon={<ArrowUpFromLine/>} name="运行" value={String(snap.Running || 0)} blue/>
            <Stat icon={<CircleCheck/>} name="成功" value={String(snap.Succeeded || 0)} good/>
            <Stat icon={<CircleX/>} name="失败" value={String(snap.Failed || 0)} bad/>
            <Stat icon={<X/>} name="取消/跳过" value={`${snap.Canceled || 0}/${snap.Skipped || 0}`}/>
          </article>
        </div>

        <article className="logs card">
          <header><h2>▼ &nbsp;详情 <span>· 实时任务</span></h2><label>{snap.Final ? resultLabel(snap.Status) : running ? '正在更新' : '等待任务'}</label></header><hr/>
          <div className="loghead"><span>状态</span><span>进度</span><span>文件 / 消息</span></div>
          {!items.length && <div className="empty-log"><FolderOpen/><span>开始任务后，这里会显示真实的文件进度和错误。</span></div>}
          {items.map((item) => <div className="log" key={item.TaskID}>
            <b className={`state-${item.Status}`}>{statusLabel(item.Status)}</b>
            <time>{item.TotalBytes > 0 ? `${Math.min(100, item.CompletedBytes * 100 / item.TotalBytes).toFixed(1)}%` : bytes(item.CompletedBytes)}</time>
            <span><strong>{item.FileName || item.SourcePath || item.TaskID}</strong>{item.Err ? ` · ${item.Err}` : item.Phase ? ` · ${item.Phase}` : ''}</span>
          </div>)}
          {(snap.Errors || []).map((message, index) => <div className="log log-error" key={`${message}-${index}`}><b>错误</b><time>—</time><span>{message}</span></div>)}
        </article>
        </>}

        <footer className="status"><span/><span>{bytes(snap.Speed)}/s&nbsp;&nbsp; | &nbsp;&nbsp;任务 {snap.Succeeded + snap.Failed + snap.Canceled}/{Math.max(snap.Succeeded + snap.Failed + snap.Canceled + snap.Pending + snap.Running, items.length)}</span></footer>
      </section>

      {chatOpen && <div className="modal-backdrop" onMouseDown={(event) => event.target === event.currentTarget && setChatOpen(false)}>
        <section className="chat-modal" role="dialog" aria-modal="true" aria-label="选择 Telegram 会话">
          <header><div><small>TELEGRAM DESTINATION</small><h2>选择会话与话题</h2></div><button aria-label="关闭" onClick={() => setChatOpen(false)}><X/></button></header>
          <div className="chat-search"><Search size={18}/><input autoFocus placeholder="搜索已加载的会话、用户名或类型" value={chatQuery} onChange={(event) => setChatQuery(event.target.value)}/></div>
          {chatError && <div className="chat-error">{chatError}</div>}
          <div className="chat-body">
            <div className="chat-list">
              {filteredChats.map((entry) => <button className={sameChat(entry, draftChat) ? 'selected' : ''} key={chatKey(entry)} onClick={() => {setDraftChat(entry); setDraftTopic(null)}}>
                <span className="chat-avatar">{entry.self ? '★' : (entry.title || entry.username || '?').slice(0, 1).toUpperCase()}</span>
                <span><b>{entry.title || entry.username || String(entry.id)}</b><small>{entry.self ? '个人收藏夹' : `${typeLabel(entry.type)}${entry.username ? ` · @${entry.username}` : ''}`}</small></span>
                {sameChat(entry, draftChat) && <Check size={18}/>}
              </button>)}
              {!filteredChats.length && !chatLoading && <p className="no-chat">没有匹配的已加载会话</p>}
              {chatNext > 0 && !chatQuery && <button className="load-more" disabled={chatLoading} onClick={() => loadChats(chatNext)}>{chatLoading ? '加载中…' : '加载更多会话'}</button>}
            </div>
            <aside className="topic-list">
              <small>当前目标</small><h3>{draftChat.title}</h3>
              {draftChat.topics.length ? <>
                <p>该会话包含话题，可选择具体目的地。</p>
                <button className={!draftTopic ? 'selected' : ''} onClick={() => setDraftTopic(null)}><span>#</span><b>不指定话题</b>{!draftTopic && <Check/>}</button>
                {draftChat.topics.map((topic) => <button className={draftTopic?.id === topic.id ? 'selected' : ''} key={topic.id} onClick={() => setDraftTopic(topic)}><span>#</span><b>{topic.title}</b>{draftTopic?.id === topic.id && <Check/>}</button>)}
              </> : <p>此会话没有可选话题，文件会直接发送到会话。</p>}
            </aside>
          </div>
          <footer><span>{chatLoading ? '正在从 Telegram 加载…' : `已加载 ${chatItems.length} 个目标`}</span><div><button onClick={() => setChatOpen(false)}>取消</button><button className="confirm" disabled={chatLoading} onClick={confirmChatSelector}>使用此目标</button></div></footer>
        </section>
      </div>}
    </main>
  );
}

function uniqueChats(items: ChatRef[]) {
  const seen = new Set<string>();
  return items.filter((item) => {
    const key = chatKey(item);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}
function chatKey(chat: ChatRef) { return chat.self ? 'self' : `${chat.type}:${chat.id}`; }
function sameChat(a: ChatRef, b: ChatRef) { return chatKey(a) === chatKey(b); }
function typeLabel(type: string) { return ({private: '私聊', group: '群组', channel: '频道', self: '收藏夹'} as Record<string, string>)[type] || type; }
function pageLabel(page: WorkspacePage) { return ({upload: '上传', download: '下载', forward: '转发', chat: '会话', tasks: '任务', backup: '备份', recover: '恢复', settings: '设置'} as Record<WorkspacePage, string>)[page]; }
function operationLabel(type: string) { return ({upload: '上传', download: '下载', forward: '转发', 'chat-export': '会话导出', backup: '备份', recover: '恢复'} as Record<string, string>)[type] || type; }
function statusLabel(status: string) { return ({queued: '排队', running: '运行', done: '完成', failed: '失败', canceled: '取消', skipped: '跳过', partial_failure: '部分失败'} as Record<string, string>)[status] || status || '等待'; }
function resultLabel(status: string) { return ({done: '任务完成', failed: '任务失败', canceled: '任务已取消', partial_failure: '任务部分失败'} as Record<string, string>)[status] || '任务结束'; }
function errorText(reason: unknown) { return reason instanceof Error ? reason.message : String(reason); }
function bytes(value = 0) { const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']; let size = Number(value) || 0; let index = 0; while (size >= 1024 && index < units.length - 1) { size /= 1024; index++; } return `${size.toFixed(1)} ${units[index]}`; }
function duration(seconds: number) { const value = Math.max(0, Math.round(seconds)); const hours = Math.floor(value / 3600); const minutes = Math.floor((value % 3600) / 60); const rest = value % 60; return hours ? `${hours}h ${minutes}m` : minutes ? `${minutes}m ${rest}s` : `${rest}s`; }
function Stat({icon, name, value, blue, good, bad}: {icon: React.ReactNode; name: string; value: string; blue?: boolean; good?: boolean; bad?: boolean}) { return <div className={`stat ${blue ? 'blue' : ''} ${good ? 'good' : ''} ${bad ? 'bad' : ''}`}>{icon}<span>{name}</span><b>{value}</b></div>; }
